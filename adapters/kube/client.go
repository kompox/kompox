package kube

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// CreateServiceAccount ensures a namespaced ServiceAccount exists (idempotent).
func (c *Client) CreateServiceAccount(ctx context.Context, namespace, name string) error {
	if c == nil || c.Clientset == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if namespace == "" {
		return fmt.Errorf("namespace is empty")
	}
	if name == "" {
		return fmt.Errorf("serviceaccount name is empty")
	}

	// Check existence first
	_, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get serviceaccount %s/%s: %w", namespace, name, err)
	}

	_, err = c.Clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create serviceaccount %s/%s: %w", namespace, name, err)
	}
	return nil
}
