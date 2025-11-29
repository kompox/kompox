package box

import (
	"context"
	"fmt"
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

// ExecInput contains parameters for executing a command in the Kompox Box Pod.
type ExecInput struct {
	// AppID is the target application id.
	AppID string `json:"app_id"`
	// Command to execute (shell-escaped already by caller as needed).
	Command []string `json:"command"`
	// TTY requests a TTY allocation.
	TTY bool `json:"tty"`
	// Stdin attaches stdin.
	Stdin bool `json:"stdin"`
	// Escape is an optional escape sequence to detach the session without sending the sequence to the remote.
	// Examples: "^P^Q", "~.", "^]", "none" to disable.
	Escape string `json:"escape"`
}

// ExecOutput may return the exit code and optional message.
type ExecOutput struct {
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
}

// Exec executes a command in the running Kompox Box Pod.
func (u *UseCase) Exec(ctx context.Context, in *ExecInput) (*ExecOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("ExecInput.AppID is required")
	}
	if len(in.Command) == 0 || strings.TrimSpace(in.Command[0]) == "" {
		return nil, fmt.Errorf("ExecInput.Command is required")
	}

	// Resolve env
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

	c := kube.NewConverter(workspaceObj, providerObj, clusterObj, appObj, "box")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.Namespace

	// Pick a running pod with the box label
	pods, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: c.SelectorString})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("kompox box pod not found")
	}
	pod := ""
	for i := range pods.Items {
		p := pods.Items[i]
		if p.DeletionTimestamp != nil {
			continue
		}
		// Prefer Ready pod
		ready := false
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == BoxContainerName && cs.Ready {
				ready = true
				break
			}
		}
		if ready || pod == "" {
			pod = p.Name
		}
	}
	if pod == "" {
		pod = pods.Items[0].Name
	}

	// Build request
	req := kcli.Clientset.CoreV1().RESTClient().Post().Resource("pods").Namespace(ns).Name(pod).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: BoxContainerName,
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

	// Execute with TTY, stdin, and escape handling via shared helper.
	result, err := terminal.RunExecStream(ctx, ex, terminal.ExecStreamOptions{
		TTY:    in.TTY,
		Stdin:  in.Stdin,
		Escape: in.Escape,
	})
	if err != nil {
		return nil, fmt.Errorf("exec stream: %w", err)
	}
	if result.Detached {
		return &ExecOutput{ExitCode: 0, Message: "detached"}, nil
	}
	return &ExecOutput{ExitCode: 0}, nil
}
