package kube

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

// PortForward sets up port forwarding to a pod and returns when ready.
func (c *Client) PortForward(ctx context.Context, opts *PortForwardOptions) (*PortForwardResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("PortForwardOptions is required")
	}
	if opts.PodName == "" {
		return nil, fmt.Errorf("PodName is required")
	}
	if opts.Namespace == "" {
		return nil, fmt.Errorf("Namespace is required")
	}
	if opts.RemotePort <= 0 {
		return nil, fmt.Errorf("RemotePort must be positive")
	}

	// Verify pod exists and is running
	pod, err := c.Clientset.CoreV1().Pods(opts.Namespace).Get(ctx, opts.PodName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", opts.Namespace, opts.PodName, err)
	}
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod %s/%s is not running (phase: %s)", opts.Namespace, opts.PodName, pod.Status.Phase)
	}

	// Find an available local port if not specified
	localPort := opts.LocalPort
	if localPort == 0 {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, fmt.Errorf("find available port: %w", err)
		}
		localPort = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	}

	// Build the port forward URL
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", opts.Namespace, opts.PodName)

	// Parse the host from RESTConfig.Host to extract just the host:port part
	var hostPort string
	if u, err := url.Parse(c.RESTConfig.Host); err == nil && u.Host != "" {
		hostPort = u.Host
	} else {
		// Fallback: assume Host is already in host:port format
		hostPort = c.RESTConfig.Host
	}

	serverURL := url.URL{Scheme: "https", Path: path, Host: hostPort}

	// Create SPDY transport
	transport, upgrader, err := spdy.RoundTripperFor(c.RESTConfig)
	if err != nil {
		return nil, fmt.Errorf("create SPDY transport: %w", err)
	}

	// Create channels for port forward communication
	readyChan := make(chan struct{})
	errorChan := make(chan error, 1)
	stopChan := make(chan struct{})

	// Set up port forward
	ports := []string{fmt.Sprintf("%d:%d", localPort, opts.RemotePort)}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &serverURL)

	fw, err := portforward.New(dialer, ports, stopChan, readyChan, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create port forwarder: %w", err)
	}

	// Start port forwarding in a goroutine
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			select {
			case errorChan <- err:
			default:
			}
		}
	}()

	// Wait for ready or error
	select {
	case <-readyChan:
		// Port forwarding is ready
		result := &PortForwardResult{
			LocalPort: localPort,
			StopFunc: func() {
				close(stopChan)
			},
		}
		return result, nil
	case err := <-errorChan:
		return nil, fmt.Errorf("port forward failed: %w", err)
	case <-ctx.Done():
		close(stopChan)
		return nil, fmt.Errorf("port forward canceled: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		close(stopChan)
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
