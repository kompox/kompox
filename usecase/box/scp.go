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

// SCPInput defines the input for the SCP operation.
type SCPInput struct {
	AppID   string   // Target app ID
	SCPArgs []string // Complete SCP arguments including source and destination
}

// SCPOutput defines the output/result of the SCP operation.
type SCPOutput struct {
	ExitCode int
	Message  string
}

// SCP transfers files to/from the Kompox Box via SCP through port forwarding.
func (u *UseCase) SCP(ctx context.Context, in *SCPInput) (*SCPOutput, error) {
	logger := logging.FromContext(ctx)

	logger.Debug(ctx, "starting SCP transfer",
		"app_id", in.AppID,
		"scp_args", strings.Join(in.SCPArgs, " "))

	// Set up port-forward to SSH daemon in Kompox Box pod
	localPort, stopFunc, err := u.setupPortForward(ctx, in.AppID, 22, 0)
	if err != nil {
		return nil, err
	}
	defer stopFunc()

	// Configure SCP to connect through the port-forwarded localhost connection
	//
	// Usage supports various SCP formats:
	//   kompoxops box scp -- localfile kompox@host:/path/to/remotefile
	//   kompoxops box scp -- kompox@host:/path/to/remotefile localfile
	//   kompoxops box scp -- -r localdir kompox@host:/path/to/remotedir
	//
	// The 'host' part (e.g., 'host', 'dummy', 'example.com') can be any arbitrary string
	// since the connection is automatically redirected to localhost via port forwarding.
	// SCP options override the hostname with localhost and the dynamically assigned port.
	scpArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "Hostname=localhost",
		"-o", "LogLevel=ERROR",
		"-P", strconv.Itoa(localPort), // Note: SCP uses -P (uppercase) for port
	}
	scpArgs = append(scpArgs, in.SCPArgs...)

	// Log the SCP command that will be executed
	cmdLine := fmt.Sprintf("scp %s", strings.Join(scpArgs, " "))
	logger.Debug(ctx, "executing SCP command",
		"local_port", localPort,
		"command", cmdLine)

	cmd := exec.CommandContext(ctx, "scp", scpArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
		logger.Error(ctx, "SCP command failed",
			"error", err,
			"exit_code", exitCode)
	} else {
		logger.Debug(ctx, "SCP command completed successfully")
	}

	return &SCPOutput{
		ExitCode: exitCode,
		Message:  "SCP transfer completed",
	}, nil
}
