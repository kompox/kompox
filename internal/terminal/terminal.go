package terminal

import (
	"context"
	"flag"
	"io"
	"os"
	"strings"

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
	ch := setupTerminalSizeUpdate(ctx, sendSize)
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
// If stdin is false, it returns nil for the reader so that no stdin is attached to the remote command.
// If escape wrapping is not necessary, it returns os.Stdin and a no-op cleanup.
func WrapStdinWithEscape(ctx context.Context, stdin, tty bool, escape string) (context.Context, *os.File, func()) {
	noop := func() {}
	if !stdin {
		return ctx, nil, noop
	}
	if !tty || escape == "" || escape == "none" {
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

// ExecStreamOptions contains parameters for streaming a remote command execution.
type ExecStreamOptions struct {
	// TTY requests a TTY allocation.
	TTY bool
	// Stdin attaches stdin.
	Stdin bool
	// Escape is an optional escape sequence to detach the session.
	// Examples: "^P^Q", "~.", "^]", "none" to disable.
	Escape string
}

// ExecStreamResult contains the result of a streamed command execution.
type ExecStreamResult struct {
	// Detached is true if the session was terminated via escape sequence.
	Detached bool
}

// RunExecStream executes a remote command via the given executor with TTY, stdin, and escape handling.
// It sets up terminal raw mode if TTY is requested, handles escape sequences, and streams I/O.
// Returns ExecStreamResult indicating whether the session was detached, or an error.
func RunExecStream(ctx context.Context, executor remotecommand.Executor, opts ExecStreamOptions) (*ExecStreamResult, error) {
	// Prepare terminal mode and window resize if TTY is requested.
	restore, sizeQueue := SetupTTYIfRequested(ctx, opts.TTY)
	if restore != nil {
		defer restore()
	}

	// Escape handling: if escape sequence is configured, wrap stdin and cancel on match.
	// WrapStdinWithEscape returns nil for stdinR when stdin is false.
	var cleanup func()
	ctx, stdinR, cleanup := WrapStdinWithEscape(ctx, opts.Stdin, opts.TTY, opts.Escape)
	if cleanup != nil {
		defer cleanup()
	}

	// Convert *os.File to io.Reader, ensuring nil stays nil (Go interface quirk:
	// a nil *os.File assigned to io.Reader is not a nil interface).
	var stdinReader io.Reader
	if stdinR != nil {
		stdinReader = stdinR
	}

	// When TTY is enabled, stderr is merged into stdout; set nil to avoid extra stream.
	var stderrW io.Writer
	if !opts.TTY {
		stderrW = os.Stderr
	}

	err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdinReader,
		Stdout:            os.Stdout,
		Stderr:            stderrW,
		Tty:               opts.TTY,
		TerminalSizeQueue: sizeQueue,
	})
	if err != nil {
		// If the user requested detach via escape, ctx will be canceled.
		if ctx.Err() == context.Canceled {
			return &ExecStreamResult{Detached: true}, nil
		}
		return nil, err
	}
	return &ExecStreamResult{Detached: false}, nil
}
