package kube

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps commonly used Kubernetes clients and the underlying REST config.
// Keep this package focused on client construction; provider-specific credential
// retrieval should live in provider drivers which then pass kubeconfig/REST config here.
type Client struct {
	// RESTConfig is the configuration used to talk to the API server.
	RESTConfig *rest.Config
	// Clientset provides typed clients for core/built-in resources.
	Clientset kubernetes.Interface
}

// Options controls client construction tuning. All fields are optional.
type Options struct {
	// UserAgent adds a custom user agent to the REST config.
	UserAgent string
	// QPS sets the allowed queries per second on the REST client.
	QPS float32
	// Burst sets the client-side rate limiter burst.
	Burst int
}

// applyDefaults applies reasonable defaults if not set.
func (o *Options) applyDefaults() {
	if o.QPS <= 0 {
		o.QPS = 20
	}
	if o.Burst <= 0 {
		o.Burst = 50
	}
}

// NewClientFromKubeconfig constructs a Client from kubeconfig bytes.
func NewClientFromKubeconfig(_ context.Context, kubeconfig []byte, opts *Options) (*Client, error) {
	if len(kubeconfig) == 0 {
		return nil, fmt.Errorf("kubeconfig is empty")
	}
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build REST config from kubeconfig: %w", err)
	}
	return NewClientFromRESTConfig(cfg, opts)
}

// NewClientFromKubeconfigPath constructs a Client from a kubeconfig file path.
func NewClientFromKubeconfigPath(ctx context.Context, path string, opts *Options) (*Client, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read kubeconfig file: %w", err)
	}
	return NewClientFromKubeconfig(ctx, data, opts)
}

// NewClientFromRESTConfig constructs a Client from an existing rest.Config.
func NewClientFromRESTConfig(cfg *rest.Config, opts *Options) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("REST config is nil")
	}
	if opts == nil {
		opts = &Options{}
	}
	opts.applyDefaults()

	cfg.QPS = opts.QPS
	cfg.Burst = opts.Burst
	if opts.UserAgent != "" {
		// AddUserAgent mutates cfg.UserAgent and returns the complete UA string.
		// We don't need the return value here.
		_ = rest.AddUserAgent(cfg, opts.UserAgent)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}

	return &Client{RESTConfig: cfg, Clientset: cs}, nil
}
