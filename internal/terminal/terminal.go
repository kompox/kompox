package terminal

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
	klog "k8s.io/klog/v2"
)

// termSizeQueue implements remotecommand.TerminalSizeQueue with a channel.
type termSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func (q *termSizeQueue) Next() *remotecommand.TerminalSize {
	sz, ok := <-q.ch
	if !ok {
		return nil
	}
	return &sz
}

// SetupTTYIfRequested sets local terminal to raw mode and starts a goroutine to publish
// terminal size updates to a queue, if tty is true. Returns a restore function (no-op if not applied)
// and a TerminalSizeQueue (nil if tty is false).
func SetupTTYIfRequested(ctx context.Context, tty bool) (restore func(), queue remotecommand.TerminalSizeQueue) {
	noop := func() {}
	restore = noop
	if !tty {
		return restore, nil
	}
	// Put local terminal to raw mode if stdin is a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		if oldState, terr := term.MakeRaw(int(os.Stdin.Fd())); terr == nil {
			restore = func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }
		}
	}
	// Prepare size queue
	sendSize := func(ch chan remotecommand.TerminalSize) {
		if w, h, e := term.GetSize(int(os.Stdin.Fd())); e == nil {
			ch <- remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}
		}
	}
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
	return restore, &termSizeQueue{ch: ch}
}

// ParseEscapeSequence converts strings like "^]", "^P^Q", or "~." into byte slices.
func ParseEscapeSequence(s string) []byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '^' && i+1 < len(s) {
			x := s[i+1]
			if x == '^' {
				out = append(out, '^')
				i += 2
				continue
			}
			u := byte(x & 31)
			if x == '?' { // DEL
				u = 0x7f
			}
			out = append(out, u)
			i += 2
			continue
		}
		if c == '~' && i+1 < len(s) && s[i+1] == '.' {
			out = append(out, '~', '.')
			i += 2
			continue
		}
		out = append(out, c)
		i++
	}
	return out
}

// WrapStdinWithEscape wraps os.Stdin to intercept a configured escape sequence when tty and stdin are enabled.
// It returns a possibly new context with cancel, the reader to pass as Stdin, and a cleanup function.
// If wrapping is not necessary, it returns the original context, os.Stdin, and a no-op cleanup.
func WrapStdinWithEscape(ctx context.Context, stdin, tty bool, escape string) (context.Context, *os.File, func()) {
	noop := func() {}
	if !(tty && stdin) || escape == "" || escape == "none" {
		return ctx, os.Stdin, noop
	}
	escBytes := ParseEscapeSequence(escape)
	if len(escBytes) == 0 {
		return ctx, os.Stdin, noop
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	pr, pw, _ := os.Pipe()
	go func() {
		defer pw.Close()
		buf := make([]byte, 1)
		matchIdx := 0
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				b := buf[0]
				if b == escBytes[matchIdx] {
					matchIdx++
					if matchIdx == len(escBytes) {
						cancel()
						return
					}
					continue
				}
				if matchIdx > 0 {
					_, _ = pw.Write(escBytes[:matchIdx])
					matchIdx = 0
				}
				_, _ = pw.Write([]byte{b})
			}
			if err != nil {
				return
			}
		}
	}()
	cleanup := func() { _ = pr.Close() }
	return ctx, pr, cleanup
}

// QuietKlog limits klog noise from k8s client-go for operations that need a quiet terminal output.
func QuietKlog() {
	klog.InitFlags(nil)
	_ = flag.Set("stderrthreshold", "FATAL")
	_ = flag.Set("v", "0")
	_ = flag.Set("logtostderr", "false")
	_ = flag.Set("alsologtostderr", "false")
}
