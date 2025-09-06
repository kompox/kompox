//go:build windows

package terminal

import (
	"context"
	"time"

	"k8s.io/client-go/tools/remotecommand"
)

// setupTerminalSizeUpdate sets up terminal size monitoring for Windows
func setupTerminalSizeUpdate(ctx context.Context, sendSize func(chan remotecommand.TerminalSize)) chan remotecommand.TerminalSize {
	ch := make(chan remotecommand.TerminalSize, 1)
	sendSize(ch)

	go func() {
		defer close(ch)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendSize(ch)
			}
		}
	}()

	return ch
}
