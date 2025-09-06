package box

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// SSHInput defines the input for the SSH operation.
type SSHInput struct {
	AppID   string   // Target app ID
	SSHArgs []string // Complete SSH arguments including hostname
}

// SSHOutput defines the output/result of the SSH operation.
type SSHOutput struct {
	ExitCode int
	Message  string
}

// SSH connects to the Kompox Box via SSH through port forwarding.
func (u *UseCase) SSH(ctx context.Context, in *SSHInput) (*SSHOutput, error) {
	logger := logging.FromContext(ctx)

	logger.Debug(ctx, "starting SSH connection",
		"app_id", in.AppID,
		"ssh_args", strings.Join(in.SSHArgs, " "))

	// Set up port-forward to SSH daemon in Kompox Box pod
	localPort, stopFunc, err := u.setupSSHPortForward(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	defer stopFunc()

	// Configure SSH to connect through the port-forwarded localhost connection
	//
	// Usage supports both user@host format and -l user option:
	//   kompoxops box ssh kompox@a
	//   kompoxops box ssh root@dummy
	//   kompoxops box ssh example.com -l user
	//
	// The 'host' part (e.g., 'a', 'dummy', 'example.com') can be any arbitrary string
	// since the connection is automatically redirected to localhost via port forwarding.
	// SSH options override the hostname with localhost and the dynamically assigned port.
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "Hostname=localhost",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(localPort),
	}
	sshArgs = append(sshArgs, in.SSHArgs...)

	// Log the SSH command that will be executed
	cmdLine := fmt.Sprintf("ssh %s", strings.Join(sshArgs, " "))
	logger.Debug(ctx, "executing SSH command",
		"local_port", localPort,
		"command", cmdLine)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		logger.Error(ctx, "SSH command failed",
			"error", err,
			"exit_code", exitCode)
	} else {
		logger.Debug(ctx, "SSH command completed successfully")
	}

	return &SSHOutput{
		ExitCode: exitCode,
		Message:  "SSH session completed",
	}, nil
}

// setupSSHPortForward sets up port forwarding to the SSH daemon in the Kompox Box pod
func (u *UseCase) setupSSHPortForward(ctx context.Context, appID string) (localPort int, stopFunc func(), err error) {
	logger := logging.FromContext(ctx)

	// Resolve environment dependencies
	appObj, err := u.Repos.App.Get(ctx, appID)
	if err != nil || appObj == nil {
		return 0, nil, fmt.Errorf("failed to get app %s: %w", appID, err)
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return 0, nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := u.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return 0, nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
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

	logger.Debug(ctx, "setting up SSH port forward",
		"namespace", ns,
		"app_name", appObj.Name,
		"cluster_name", clusterObj.Name)

	// Find the Kompox Box pod
	pod, err := kcli.FindPodByLabels(ctx, ns, LabelBoxSelector)
	if err != nil {
		return 0, nil, fmt.Errorf("find Kompox Box pod: %w", err)
	}

	logger.Debug(ctx, "found kompox box pod",
		"pod_name", pod.Name,
		"pod_phase", pod.Status.Phase,
		"pod_node", pod.Spec.NodeName)

	// Set up port forwarding to SSH port (22)
	result, err := kcli.PortForward(ctx, &kube.PortForwardOptions{
		Namespace:  ns,
		PodName:    pod.Name,
		LocalPort:  0,  // auto-assign
		RemotePort: 22, // SSH daemon port
	})
	if err != nil {
		return 0, nil, fmt.Errorf("setup SSH port forward: %w", err)
	}

	logger.Debug(ctx, "SSH port forward established",
		"local_port", result.LocalPort,
		"remote_port", 22,
		"pod_name", pod.Name,
		"namespace", ns)

	return result.LocalPort, result.StopFunc, nil
}
