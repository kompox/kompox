package tool

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	// Terminal raw mode utils
	"golang.org/x/term"
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

// ExecInput contains parameters for executing a command in the runner Pod.
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

// Exec executes a command in the running maintenance runner Pod.
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
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
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

	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.NSName

	// Pick a running pod
	pods, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "kompox.dev/tool-runner=true"})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("runner pod not found")
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
			if cs.Name == "runner" && cs.Ready {
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
		Container: "runner",
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
	// Prepare terminal mode and window resize if TTY is requested
	var restore func()
	var sizeQueue remotecommand.TerminalSizeQueue
	if in.TTY {
		// Put local terminal to raw mode so input is not echoed and signals are passed through
		if term.IsTerminal(int(os.Stdin.Fd())) {
			oldState, terr := term.MakeRaw(int(os.Stdin.Fd()))
			if terr == nil {
				restore = func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }
				// Ensure restore on context end
				defer func() {
					if restore != nil {
						restore()
					}
				}()
			}
		}
		// Handle window resize events to keep remote PTY size synced
		// initial size
		sendSize := func(ch chan remotecommand.TerminalSize) {
			if w, h, e := term.GetSize(int(os.Stdin.Fd())); e == nil {
				ch <- remotecommand.TerminalSize{Width: uint16(w), Height: uint16(h)}
			}
		}
		ch := make(chan remotecommand.TerminalSize, 1)
		sendSize(ch)
		// Listen for SIGWINCH
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
					// periodic refresh for safety
					sendSize(ch)
				}
			}
		}()
		sizeQueue = &termSizeQueue{ch: ch}
	}

	// Escape handling: if escape sequence is configured, wrap stdin and cancel on match
	var stdinR *os.File
	if in.Stdin {
		stdinR = os.Stdin
	}
	if in.TTY && in.Stdin && in.Escape != "" && in.Escape != "none" {
		escBytes := parseEscapeSequence(in.Escape)
		if len(escBytes) > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			pr, pw, _ := os.Pipe()
			// Tee stdin to pipe while intercepting escape
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
								// Detach: do not forward escape to remote, cancel context
								cancel()
								return
							}
							continue
						}
						// flush any partially matched bytes before current
						if matchIdx > 0 {
							pw.Write(escBytes[:matchIdx])
							matchIdx = 0
						}
						pw.Write([]byte{b})
					}
					if err != nil {
						return
					}
				}
			}()
			stdinR = pr
			// ensure pipe reader closed when done
			defer func() { _ = pr.Close() }()
		}
	}

	// Stream to stdio (could be customized by caller later)
	var stderrW io.Writer
	if !in.TTY {
		stderrW = os.Stderr
	} // when TTY=true, stderr is merged into stdout; set nil to avoid extra stream
	if err := ex.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdinR,
		Stdout:            os.Stdout,
		Stderr:            stderrW,
		Tty:               in.TTY,
		TerminalSizeQueue: sizeQueue,
	}); err != nil {
		// If the user requested detach via escape, our ctx will be canceled.
		if ctx.Err() == context.Canceled {
			return &ExecOutput{ExitCode: 0, Message: "detached"}, nil
		}
		return nil, fmt.Errorf("exec stream: %w", err)
	}
	return &ExecOutput{ExitCode: 0}, nil
}

// parseEscapeSequence converts strings like "^]", "^P^Q", or "~." into byte slices.
func parseEscapeSequence(s string) []byte {
	// Trim spaces
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Support caret notation for control chars: ^A = 0x01, ^] = 0x1d, etc.
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
			// Map @ = NUL, [A-Z\x5B-\x5F] typical; we'll do generic: (x & 0x1F)
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
