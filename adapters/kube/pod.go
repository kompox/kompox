package kube

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// PodLogInput defines parameters for fetching pod logs.
type PodLogInput struct {
	Namespace string
	Pod       string
	Container string
	Follow    bool
	TailLines *int64
}

// PodLogOutput is a placeholder for future extension.
type PodLogOutput struct{}

// PodLog streams or prints logs from a specified pod.
func (c *Client) PodLog(ctx context.Context, in *PodLogInput) (*PodLogOutput, error) {
	if in == nil || in.Namespace == "" || in.Pod == "" {
		return nil, fmt.Errorf("namespace and pod name are required")
	}
	opts := &corev1.PodLogOptions{Container: in.Container, Follow: in.Follow}
	if in.TailLines != nil && *in.TailLines > 0 {
		opts.TailLines = in.TailLines
	}
	connCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	logReq := c.Clientset.CoreV1().Pods(in.Namespace).GetLogs(in.Pod, opts)
	stream, err := logReq.Stream(connCtx)
	if err != nil {
		return nil, fmt.Errorf("get logs stream: %w", err)
	}
	defer stream.Close()
	if in.Follow {
		reader := bufio.NewReader(stream)
		for {
			select {
			case <-ctx.Done():
				return &PodLogOutput{}, nil
			default:
			}
			line, e := reader.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = os.Stdout.Write(line)
			}
			if e != nil {
				if e == io.EOF {
					return &PodLogOutput{}, nil
				}
				return nil, fmt.Errorf("read logs: %w", e)
			}
		}
	}
	if _, err := io.Copy(os.Stdout, stream); err != nil {
		return nil, fmt.Errorf("copy logs: %w", err)
	}
	return &PodLogOutput{}, nil
}
