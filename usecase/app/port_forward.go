package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

type warnLineWriter struct {
	ctx    context.Context
	logger logging.Logger
	msg    string
	buf    []byte
}

func (w *warnLineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:i]))
		if line != "" {
			// Keep msg as a stable symbol; put the actual error line into err.
			w.logger.Warn(w.ctx, w.msg, "err", line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}

// PortForwardInput defines parameters for port-forwarding to an app (or box) pod.
type PortForwardInput struct {
	AppID string `json:"app_id"`

	// Component is the target component label value.
	// Defaults to "app". Set to "box" to connect to Kompox Box.
	Component string `json:"component"`

	// Service is an optional Compose service name.
	// It is only meaningful when Component is "app".
	Service string `json:"service"`

	// Address is a comma-separated list of bind addresses (default: "localhost").
	Address string `json:"address"`

	// Ports is a list of port specs in one of the following forms:
	//   - "LOCAL:REMOTE"
	//   - "PORT" (same local/remote)
	//   - ":REMOTE" (auto-assign local port)
	Ports []string `json:"ports"`
}

// PortForwardOutput is reserved for future use.
//
// Port-forward is a blocking operation; runtime details (such as assigned local ports)
// are not available to the caller until the operation ends.
// To keep the usecase API consistent, this output is currently returned empty.
type PortForwardOutput struct{}

func parsePortSpec(s string) (localPort int, remotePort int, err error) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return 0, 0, fmt.Errorf("port spec is empty")
	}

	if strings.Contains(ss, ":") {
		parts := strings.SplitN(ss, ":", 2)
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid port spec: %q", s)
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if right == "" {
			return 0, 0, fmt.Errorf("remote port is required: %q", s)
		}

		if left == "" {
			localPort = 0
		} else {
			lp, perr := strconv.Atoi(left)
			if perr != nil {
				return 0, 0, fmt.Errorf("invalid local port %q: %w", left, perr)
			}
			if lp < 0 {
				return 0, 0, fmt.Errorf("local port must be non-negative: %d", lp)
			}
			localPort = lp
		}

		rp, perr := strconv.Atoi(right)
		if perr != nil {
			return 0, 0, fmt.Errorf("invalid remote port %q: %w", right, perr)
		}
		if rp <= 0 {
			return 0, 0, fmt.Errorf("remote port must be positive: %d", rp)
		}
		remotePort = rp
		return localPort, remotePort, nil
	}

	p, perr := strconv.Atoi(ss)
	if perr != nil {
		return 0, 0, fmt.Errorf("invalid port %q: %w", ss, perr)
	}
	if p <= 0 {
		return 0, 0, fmt.Errorf("port must be positive: %d", p)
	}
	return p, p, nil
}

func splitAddresses(s string) []string {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return []string{"localhost"}
	}
	var out []string
	for _, part := range strings.Split(ss, ",") {
		a := strings.TrimSpace(part)
		if a == "" {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return []string{"localhost"}
	}
	return out
}

func isExpectedPortForwardStop(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "port forward ended") || strings.Contains(msg, "use of closed network connection") || strings.Contains(msg, "EOF")
}

func isRecoverablePortForwardError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// When the remote port isn't actually listening, client-go may surface it as
	// "lost connection to pod" (and also emit a separate Unhandled Error line).
	if strings.Contains(msg, "connection refused") {
		return true
	}
	if strings.Contains(msg, "lost connection to pod") {
		return true
	}
	return false
}

// PortForward establishes port-forwarding to a target pod and blocks until the session ends.
// The session ends when:
//   - The caller cancels the context (Ctrl+C), or
//   - The target pod terminates.
func (u *UseCase) PortForward(ctx context.Context, in *PortForwardInput) (*PortForwardOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("PortForwardInput.AppID is required")
	}
	if len(in.Ports) == 0 {
		return nil, fmt.Errorf("at least one port spec is required")
	}

	component := strings.TrimSpace(in.Component)
	if component == "" {
		component = "app"
	}
	addresses := splitAddresses(in.Address)

	logger := logging.FromContext(ctx)
	msgSym := "UC:app.port-forward"

	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil || appObj == nil {
		return nil, fmt.Errorf("failed to get app %s: %w", in.AppID, err)
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := u.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var workspaceObj *model.Workspace
	if providerObj.WorkspaceID != "" {
		workspaceObj, _ = u.Repos.Workspace.Get(ctx, providerObj.WorkspaceID)
	}

	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(workspaceObj, providerObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	c := kube.NewConverter(workspaceObj, providerObj, clusterObj, appObj, component)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}

	// Validate/resolve service (only meaningful for component=app).
	resolvedService := strings.TrimSpace(in.Service)
	if component == "app" {
		if c.Project == nil || len(c.Project.Services) == 0 {
			return nil, fmt.Errorf("compose project has no services")
		}
		if resolvedService == "" {
			serviceNames := make([]string, 0, len(c.Project.Services))
			for name := range c.Project.Services {
				serviceNames = append(serviceNames, name)
			}
			sort.Strings(serviceNames)
			resolvedService = serviceNames[0]
		} else {
			if _, ok := c.Project.Services[resolvedService]; !ok {
				return nil, fmt.Errorf("service %q not found in compose project", resolvedService)
			}
		}
	}

	ns := c.Namespace

	var ports []kube.PortForwardPort
	var remotePorts []int
	for _, ps := range in.Ports {
		lp, rp, err := parsePortSpec(ps)
		if err != nil {
			return nil, err
		}
		ports = append(ports, kube.PortForwardPort{LocalPort: lp, RemotePort: rp})
		remotePorts = append(remotePorts, rp)
	}

	pfLogger := logger.With(
		"namespace", ns,
	)

	pfLogger.Info(ctx, msgSym+":Start",
		"component", component,
		"service", resolvedService,
		"addresses", addresses,
		"ports", in.Ports,
		"remotePorts", remotePorts,
	)

	backoff := time.Second
	for {
		pfLogger.Info(ctx, msgSym+":SelectPod/s")
		pod, err := kcli.FindPodByLabels(ctx, ns, c.SelectorString)
		if err != nil {
			return nil, fmt.Errorf("find target pod: %w", err)
		}
		pfLogger.Info(ctx, msgSym+":SelectPod/eok", "podName", pod.Name)

		pf, err := kcli.PortForwardMulti(ctx, &kube.PortForwardMultiOptions{
			Namespace: ns,
			PodName:   pod.Name,
			Ports:     ports,
			Addresses: addresses,
			ErrOut:    &warnLineWriter{ctx: ctx, logger: pfLogger, msg: msgSym + ":ForwardError"},
		})
		if err != nil {
			return nil, err
		}
		pfLogger.Info(ctx, msgSym+":Established",
			"podName", pod.Name,
			"addresses", addresses,
			"localPorts", pf.LocalPorts,
			"remotePorts", remotePorts,
		)
		backoff = time.Second

		select {
		case <-ctx.Done():
			pf.StopFunc()
			<-pf.DoneChannel
			return &PortForwardOutput{}, nil
		case <-pf.DoneChannel:
			var ferr error
			select {
			case ferr = <-pf.ErrorChannel:
			default:
			}
			pf.StopFunc()
			if ferr == nil || isExpectedPortForwardStop(ferr) {
				return &PortForwardOutput{}, nil
			}
			if isRecoverablePortForwardError(ferr) {
				pfLogger.Warn(ctx, msgSym+":Retrying", "err", ferr.Error())
				// Backoff to avoid a tight restart loop when the user keeps hitting an unopened port.
				select {
				case <-ctx.Done():
					return &PortForwardOutput{}, nil
				case <-time.After(backoff):
				}
				if backoff < 5*time.Second {
					backoff *= 2
					if backoff > 5*time.Second {
						backoff = 5 * time.Second
					}
				}
				continue
			}
			return nil, fmt.Errorf("port forward ended: %w", ferr)
		}
	}
}
