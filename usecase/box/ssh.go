package box

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

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
	localPort, stopFunc, err := u.setupPortForward(ctx, in.AppID, 22, 0)
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
