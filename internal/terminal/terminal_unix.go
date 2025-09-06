//go:build !windows

package terminal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/tools/remotecommand"
)

// setupTerminalSizeUpdate sets up terminal size monitoring for Unix-like systems
func setupTerminalSizeUpdate(ctx context.Context, sendSize func(chan remotecommand.TerminalSize)) chan remotecommand.TerminalSize {
	ch := make(chan remotecommand.TerminalSize, 1)
	sendSize(ch)
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, syscall.SIGWINCH)

	go func() {
		defer close(ch)
		defer signal.Stop(sigch)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigch:
				sendSize(ch)
			case <-ticker.C:
				sendSize(ch)
			}
		}
	}()

	return ch
}
