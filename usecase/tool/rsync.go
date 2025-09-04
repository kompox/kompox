package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// RsyncInput defines the input for the rsync operation.
type RsyncInput struct {
	AppID     string   // Target app ID
	RsyncArgs []string // Complete rsync arguments including paths with vol: prefixes
}

// RsyncOutput defines the output/result of the rsync operation.
type RsyncOutput struct {
	Stdout string
	Stderr string
}

// portForwardSession holds active port forward session info
type portForwardSession struct {
	localPort int
	stopFunc  func()
}

var (
	// activePortForwards tracks active port forward sessions by appID
	activePortForwards = make(map[string]*portForwardSession)
	portForwardMutex   sync.Mutex
)

// Rsync performs rsync between local and remote (via port-forwarded rsyncd)
func (uc *UseCase) Rsync(ctx context.Context, in *RsyncInput) (*RsyncOutput, error) {
	logger := logging.FromContext(ctx)

	logger.Info(ctx, "starting rsync operation",
		"app_id", in.AppID,
		"rsync_args", strings.Join(in.RsyncArgs, " "))

	// Check if any arguments contain vol: prefixes
	hasVolPaths := false
	for _, arg := range in.RsyncArgs {
		if strings.HasPrefix(arg, "vol:") {
			hasVolPaths = true
			break
		}
	}

	if !hasVolPaths {
		return nil, fmt.Errorf("no vol: paths found in rsync arguments")
	}

	// Set up port-forward to rsync daemon in tool runner pod
	localPort, stopFunc, err := uc.setupRsyncPortForward(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	defer stopFunc()

	// Transform vol: paths to rsync URLs
	transformedArgs := make([]string, len(in.RsyncArgs))
	for i, arg := range in.RsyncArgs {
		if strings.HasPrefix(arg, "vol:") {
			// Convert vol:/path or vol: to rsync://localhost:<localPort>/vol/path
			remotePath := strings.TrimPrefix(arg, "vol:")
			if remotePath == "" {
				remotePath = "/"
			} else if !strings.HasPrefix(remotePath, "/") {
				remotePath = "/" + remotePath
			}
			transformedArgs[i] = fmt.Sprintf("rsync://localhost:%d/vol%s", localPort, remotePath)
		} else {
			transformedArgs[i] = arg
		}
	}

	// Log the rsync command that will be executed
	cmdLine := fmt.Sprintf("rsync %s", strings.Join(transformedArgs, " "))
	logger.Info(ctx, "executing rsync command",
		"local_port", localPort,
		"command", cmdLine)

	cmd := exec.CommandContext(ctx, "rsync", transformedArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	if err != nil {
		logger.Error(ctx, "rsync command failed",
			"error", err,
			"exit_code", cmd.ProcessState.ExitCode(),
			"stderr", stderr.String())
	} else {
		logger.Info(ctx, "rsync command completed successfully",
			"stdout_size", stdout.Len(),
			"stderr_size", stderr.Len())
	}

	return &RsyncOutput{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, err
}

// setupRsyncPortForward sets up port forwarding to the rsync daemon in the tool runner pod
func (uc *UseCase) setupRsyncPortForward(ctx context.Context, appID string) (localPort int, stopFunc func(), err error) {
	logger := logging.FromContext(ctx)

	// Resolve environment dependencies
	appObj, err := uc.Repos.App.Get(ctx, appID)
	if err != nil || appObj == nil {
		return 0, nil, fmt.Errorf("failed to get app %s: %w", appID, err)
	}
	clusterObj, err := uc.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return 0, nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := uc.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return 0, nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = uc.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	// Create provider driver to get kubeconfig
	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return 0, nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Get namespace using converter
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj)
	if _, err := c.Convert(ctx); err != nil {
		return 0, nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.NSName

	logger.Info(ctx, "setting up port forward",
		"namespace", ns,
		"app_name", appObj.Name,
		"cluster_name", clusterObj.Name)

	// Find the tool runner pod
	pod, err := kcli.FindPodByLabels(ctx, ns, "kompox.dev/tool-runner=true")
	if err != nil {
		return 0, nil, fmt.Errorf("find tool runner pod: %w", err)
	}

	logger.Info(ctx, "found tool runner pod",
		"pod_name", pod.Name,
		"pod_phase", pod.Status.Phase,
		"pod_node", pod.Spec.NodeName)

	// Set up port forwarding
	result, err := kcli.PortForward(ctx, &kube.PortForwardOptions{
		Namespace:  ns,
		PodName:    pod.Name,
		LocalPort:  0,   // auto-assign
		RemotePort: 873, // rsync daemon port
	})
	if err != nil {
		return 0, nil, fmt.Errorf("setup port forward: %w", err)
	}

	logger.Info(ctx, "port forward established",
		"local_port", result.LocalPort,
		"remote_port", 873,
		"pod_name", pod.Name,
		"namespace", ns)

	return result.LocalPort, result.StopFunc, nil
}
