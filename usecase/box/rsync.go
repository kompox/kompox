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

// RsyncInput defines the input for the Rsync operation.
type RsyncInput struct {
	AppID     string   // Target app ID
	RsyncArgs []string // Complete rsync arguments including source and destination
}

// RsyncOutput defines the output/result of the Rsync operation.
type RsyncOutput struct {
	ExitCode int
	Message  string
}

// Rsync synchronizes files to/from the Kompox Box via rsync through port forwarding.
func (u *UseCase) Rsync(ctx context.Context, in *RsyncInput) (*RsyncOutput, error) {
	logger := logging.FromContext(ctx)

	logger.Debug(ctx, "starting rsync synchronization",
		"app_id", in.AppID,
		"rsync_args", strings.Join(in.RsyncArgs, " "))

	// Set up port-forward to SSH daemon in Kompox Box pod
	localPort, stopFunc, err := u.setupPortForward(ctx, in.AppID, 22, 0)
	if err != nil {
		return nil, err
	}
	defer stopFunc()

	// Configure SSH arguments for rsync's -e option
	//
	// Usage supports various rsync formats:
	//   kompoxops box rsync -- -avz localdir/ root@host:/path/to/remotedir/
	//   kompoxops box rsync -- -avz root@host:/path/to/remotedir/ localdir/
	//
	// The 'host' part (e.g., 'host', 'dummy', 'example.com') can be any arbitrary string
	// since the connection is automatically redirected to localhost via port forwarding.
	// SSH options override the hostname with localhost and the dynamically assigned port.
	sshArgs := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "Hostname=localhost",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(localPort),
	}

	// Build rsync command with -e option for SSH configuration
	rsyncArgs := []string{
		"-e", strings.Join(sshArgs, " "),
	}
	rsyncArgs = append(rsyncArgs, in.RsyncArgs...)

	// Log the rsync command that will be executed
	cmdLine := fmt.Sprintf("rsync %s", strings.Join(rsyncArgs, " "))
	logger.Debug(ctx, "executing rsync command",
		"local_port", localPort,
		"command", cmdLine)

	cmd := exec.CommandContext(ctx, "rsync", rsyncArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		logger.Error(ctx, "rsync command failed",
			"error", err,
			"exit_code", exitCode)
	} else {
		logger.Debug(ctx, "rsync command completed successfully")
	}

	return &RsyncOutput{
		ExitCode: exitCode,
		Message:  "rsync synchronization completed",
	}, nil
}
