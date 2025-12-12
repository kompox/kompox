package kube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardOptions configures port forwarding behavior.
type PortForwardOptions struct {
	// Namespace of the target pod
	Namespace string
	// PodName is the name of the target pod
	PodName string
	// LocalPort is the local port to bind to (0 for auto-assignment)
	LocalPort int
	// RemotePort is the remote port to forward to
	RemotePort int
	// Addresses is the list of addresses to listen on (default: ["localhost"]).
	// This maps to kubectl's --address behavior.
	Addresses []string
	// ReadyChannel receives the local port once forwarding is ready
	ReadyChannel chan struct{}
	// ErrorChannel receives any errors during forwarding
	ErrorChannel chan error
}

// PortForwardResult contains the result of port forwarding setup.
type PortForwardResult struct {
	// LocalPort is the actual local port being used
	LocalPort int
	// StopFunc can be called to stop port forwarding
	StopFunc func()
	// DoneChannel is closed when the port forwarder stops.
	DoneChannel <-chan struct{}
	// ErrorChannel receives the first forwarding error, if any.
	ErrorChannel <-chan error
}

// PortForwardPort is a pair of local/remote ports for port-forwarding.
// LocalPort can be 0 for auto-assignment.
type PortForwardPort struct {
	LocalPort  int
	RemotePort int
}

// PortForwardMultiOptions configures multi-port forwarding behavior.
type PortForwardMultiOptions struct {
	Namespace string
	PodName   string
	Ports     []PortForwardPort
	// Addresses is the list of addresses to listen on (default: ["localhost"]).
	Addresses []string
	// Out is an optional destination for normal output.
	// When nil, output is discarded.
	Out io.Writer
	// ErrOut is an optional destination for per-connection forwarding errors.
	// Typical examples are "connection refused" when the remote port isn't listening.
	// When nil, output is discarded.
	ErrOut       io.Writer
	ReadyChannel chan struct{}
	ErrorChannel chan error
}

// PortForwardMultiResult contains the result of multi port-forwarding setup.
type PortForwardMultiResult struct {
	LocalPorts   []int
	StopFunc     func()
	DoneChannel  <-chan struct{}
	ErrorChannel <-chan error
}

// PortForward sets up port forwarding to a pod and returns when ready.
func (c *Client) PortForward(ctx context.Context, opts *PortForwardOptions) (*PortForwardResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("PortForwardOptions is required")
	}
	res, err := c.PortForwardMulti(ctx, &PortForwardMultiOptions{
		Namespace:    opts.Namespace,
		PodName:      opts.PodName,
		Ports:        []PortForwardPort{{LocalPort: opts.LocalPort, RemotePort: opts.RemotePort}},
		Addresses:    opts.Addresses,
		ReadyChannel: opts.ReadyChannel,
		ErrorChannel: opts.ErrorChannel,
	})
	if err != nil {
		return nil, err
	}
	localPort := 0
	if len(res.LocalPorts) > 0 {
		localPort = res.LocalPorts[0]
	}
	return &PortForwardResult{
		LocalPort:    localPort,
		StopFunc:     res.StopFunc,
		DoneChannel:  res.DoneChannel,
		ErrorChannel: res.ErrorChannel,
	}, nil
}

func normalizeAddresses(in []string) []string {
	var out []string
	for _, raw := range in {
		for _, part := range strings.Split(raw, ",") {
			a := strings.TrimSpace(part)
			if a == "" {
				continue
			}
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return []string{"localhost"}
	}
	return out
}

func isTerminalPortForwardError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "EOF") || strings.Contains(msg, "use of closed network connection")
}

// PortForwardMulti sets up multi-port forwarding to a pod and returns when ready.
func (c *Client) PortForwardMulti(ctx context.Context, opts *PortForwardMultiOptions) (*PortForwardMultiResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("PortForwardMultiOptions is required")
	}
	if opts.PodName == "" {
		return nil, fmt.Errorf("PodName is required")
	}
	if opts.Namespace == "" {
		return nil, fmt.Errorf("Namespace is required")
	}
	if len(opts.Ports) == 0 {
		return nil, fmt.Errorf("Ports is required")
	}
	for _, p := range opts.Ports {
		if p.RemotePort <= 0 {
			return nil, fmt.Errorf("RemotePort must be positive")
		}
		if p.LocalPort < 0 {
			return nil, fmt.Errorf("LocalPort must be non-negative")
		}
	}

	// Verify pod exists and is running
	pod, err := c.Clientset.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", opts.Namespace, opts.PodName, err)
	}
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod %s/%s is not running (phase: %s)", opts.Namespace, opts.PodName, pod.Status.Phase)
	}

	assignedLocalPorts := make([]int, 0, len(opts.Ports))
	portSpecs := make([]string, 0, len(opts.Ports))
	seenLocal := map[int]struct{}{}
	for _, p := range opts.Ports {
		localPort := p.LocalPort
		if localPort == 0 {
			listener, err := net.Listen("tcp", ":0")
			if err != nil {
				return nil, fmt.Errorf("find available port: %w", err)
			}
			localPort = listener.Addr().(*net.TCPAddr).Port
			listener.Close()
		}
		if _, dup := seenLocal[localPort]; dup {
			return nil, fmt.Errorf("duplicate local port: %d", localPort)
		}
		seenLocal[localPort] = struct{}{}
		assignedLocalPorts = append(assignedLocalPorts, localPort)
		portSpecs = append(portSpecs, fmt.Sprintf("%d:%d", localPort, p.RemotePort))
	}

	addresses := normalizeAddresses(opts.Addresses)

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", opts.Namespace, opts.PodName)
	var hostPort string
	if u, err := url.Parse(c.RESTConfig.Host); err == nil && u.Host != "" {
		hostPort = u.Host
	} else {
		hostPort = c.RESTConfig.Host
	}
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostPort}

	transport, upgrader, err := spdy.RoundTripperFor(c.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("create SPDY transport: %w", err)
	}

	readyChan := make(chan struct{})
	errChan := make(chan error, 1)
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() { close(stopChan) })
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &serverURL)

	outW := opts.Out
	errW := opts.ErrOut
	if outW == nil {
		outW = io.Discard
	}
	if errW == nil {
		errW = io.Discard
	}

	fw, err := portforward.NewOnAddresses(dialer, addresses, portSpecs, stopChan, readyChan, outW, errW)
	if err != nil {
		return nil, fmt.Errorf("create port forwarder: %w", err)
	}

	go func() {
		defer close(doneChan)
		// client-go's portforward implementation may emit per-connection errors via
		// utilruntime.HandleError ("Unhandled Error" by default). When the caller
		// provided ErrOut, route those to ErrOut instead of printing to stderr.
		//
		// Note: utilruntime.ErrorHandlers is process-global.
		var restoreRuntimeErrorHandlers func()
		if opts.ErrOut != nil {
			prev := utilruntime.ErrorHandlers
			utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
				func(_ context.Context, err error, msg string, _ ...any) {
					if msg == "" {
						msg = "Unhandled Error"
					}
					_, _ = fmt.Fprintf(errW, "%s: %v\n", msg, err)
				},
			}
			restoreRuntimeErrorHandlers = func() { utilruntime.ErrorHandlers = prev }
		}
		if restoreRuntimeErrorHandlers != nil {
			defer restoreRuntimeErrorHandlers()
		}
		if err := fw.ForwardPorts(); err != nil {
			select {
			case errChan <- err:
			default:
			}
			if opts.ErrorChannel != nil {
				select {
				case opts.ErrorChannel <- err:
				default:
				}
			}
		}
	}()

	go func() {
		// Best-effort stop when the target pod terminates.
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-doneChan:
				return
			case <-ticker.C:
				p, err := c.Clientset.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
				if err != nil {
					stop()
					return
				}
				if p.DeletionTimestamp != nil || p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
					stop()
					return
				}
			}
		}
	}()

	select {
	case <-readyChan:
		if opts.ReadyChannel != nil {
			select {
			case opts.ReadyChannel <- struct{}{}:
			default:
			}
		}
		return &PortForwardMultiResult{
			LocalPorts:   assignedLocalPorts,
			StopFunc:     stop,
			DoneChannel:  doneChan,
			ErrorChannel: errChan,
		}, nil
	case err := <-errChan:
		stop()
		if isTerminalPortForwardError(err) {
			return nil, fmt.Errorf("port forward ended")
		}
		return nil, fmt.Errorf("port forward failed: %w", err)
	case <-ctx.Done():
		stop()
		return nil, fmt.Errorf("port forward canceled: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		stop()
		return nil, fmt.Errorf("port forward timeout")
	}
}

// FindPodByLabels finds a pod in the given namespace matching the label selector.
// Returns the first pod found that is either Ready or at least not terminated.
func (c *Client) FindPodByLabels(ctx context.Context, namespace, labelSelector string) (*corev1.Pod, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if labelSelector == "" {
		return nil, fmt.Errorf("labelSelector is required")
	}

	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found with selector %s", labelSelector)
	}

	// Prefer Ready pod, then non-terminated pod
	var readyPod, nonTerminatedPod *corev1.Pod
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Check if pod is ready
		ready := false
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready = true
				break
			}
		}

		if ready {
			readyPod = pod
			break
		} else if nonTerminatedPod == nil && pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
			nonTerminatedPod = pod
		}
	}

	if readyPod != nil {
		return readyPod, nil
	}
	if nonTerminatedPod != nil {
		return nonTerminatedPod, nil
	}

	return &pods.Items[0], nil
}
