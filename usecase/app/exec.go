package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/terminal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// termSizeQueue implements remotecommand.TerminalSizeQueue with a channel.

// ExecInput defines parameters to execute a command in one of app pods.
type ExecInput struct {
	// AppID is the target application id.
	AppID string `json:"app_id"`
	// Command to execute (shell-escaped already by caller as needed).
	Command []string `json:"command"`
	// TTY requests a TTY allocation.
	TTY bool `json:"tty"`
	// Stdin attaches stdin.
	Stdin bool `json:"stdin"`
	// Container is an optional container name. If empty, the first container is used.
	Container string `json:"container"`
	// Escape is an optional escape sequence to detach the session without sending the sequence to the remote.
	// Examples: "^P^Q", "~.", "^]", "none" to disable.
	Escape string `json:"escape"`
}

// ExecOutput returns the exit code and optional message.
type ExecOutput struct {
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
}

// Exec executes a command in a running Pod of the app's namespace.
// Selection strategy:
//   - Determine app namespace via kube converter
//   - List pods in the namespace, skip those labeled as tool-runner
//   - Prefer a pod that has at least one Ready container; fallback to the first non-terminating pod
//   - Use specified container name if provided, otherwise first container
func (u *UseCase) Exec(ctx context.Context, in *ExecInput) (*ExecOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("ExecInput.AppID is required")
	}
	if len(in.Command) == 0 || strings.TrimSpace(in.Command[0]) == "" {
		return nil, fmt.Errorf("ExecInput.Command is required")
	}

	// Resolve app environment
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
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
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

	// Compute namespace via converter
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.NSName

	// Find target pod
	podsList, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil || len(podsList.Items) == 0 {
		return nil, fmt.Errorf("app pod not found")
	}
	podName := ""
	// Pick a pod: prefer Ready and not tool-runner
	for i := range podsList.Items {
		p := podsList.Items[i]
		if p.DeletionTimestamp != nil {
			continue
		}
		if p.Labels["kompox.dev/tool-runner"] == "true" {
			continue
		}
		ready := false
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready = true
				break
			}
		}
		if ready || podName == "" {
			podName = p.Name
		}
	}
	if podName == "" {
		// fallback to the first non-terminating non tool-runner pod
		for i := range podsList.Items {
			p := podsList.Items[i]
			if p.DeletionTimestamp == nil && p.Labels["kompox.dev/tool-runner"] != "true" {
				podName = p.Name
				break
			}
		}
	}
	if podName == "" {
		// fallback to the first item
		podName = podsList.Items[0].Name
	}

	// Build exec request
	req := kcli.Clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(ns).Name(podName).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: in.Container,
		Command:   in.Command,
		Stdin:     in.Stdin,
		Stdout:    true,
		Stderr:    true,
		TTY:       in.TTY,
	}, scheme.ParameterCodec)

	ex, err := remotecommand.NewSPDYExecutor(kcli.RESTConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	// Prepare terminal mode and window resize if TTY is requested
	restore, sizeQueue := terminal.SetupTTYIfRequested(ctx, in.TTY)
	if restore != nil {
		defer restore()
	}

	// Escape handling: if escape sequence is configured, wrap stdin and cancel on match
	var stdinR *os.File
	if in.Stdin {
		stdinR = os.Stdin
	}
	var cleanup func()
	ctx, stdinR, cleanup = terminal.WrapStdinWithEscape(ctx, in.Stdin, in.TTY, in.Escape)
	if cleanup != nil {
		defer cleanup()
	}

	var stderrW io.Writer
	if !in.TTY {
		stderrW = os.Stderr
	}
	if err := ex.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdinR,
		Stdout:            os.Stdout,
		Stderr:            stderrW,
		Tty:               in.TTY,
		TerminalSizeQueue: sizeQueue,
	}); err != nil {
		if ctx.Err() == context.Canceled {
			return &ExecOutput{ExitCode: 0, Message: "detached"}, nil
		}
		return nil, fmt.Errorf("exec stream: %w", err)
	}
	return &ExecOutput{ExitCode: 0}, nil
}

// parseEscapeSequence removed in favor of internal/termexec.ParseEscapeSequence
