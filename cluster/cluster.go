package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yaegashi/kompoxops/models/cfgops"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Cluster wraps Kubernetes clients and metadata derived from cfgops.Root.
type Cluster struct {
	Config   *cfgops.Root
	Rest     *rest.Config
	Client   *kubernetes.Clientset
	Context  string
	Kubeconf string
}

// New constructs a Cluster from kompoxops cfg. It builds rest.Config using kubeconfig/context when provided.
func New(cfg *cfgops.Root) (*Cluster, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	ca := cfg.Cluster.Auth

	// Determine kubeconfig path (expand ~ to $HOME)
	kubeconfig := ca.Kubeconfig
	if kubeconfig == "" {
		if env := os.Getenv("KUBECONFIG"); env != "" {
			kubeconfig = env
		} else {
			home, _ := os.UserHomeDir()
			if home != "" {
				kubeconfig = filepath.Join(home, ".kube", "config")
			}
		}
	} else if kubeconfig[0] == '~' {
		// Expand leading ~ in provided path
		home, _ := os.UserHomeDir()
		if home != "" {
			kubeconfig = filepath.Join(home, kubeconfig[1:])
		}
	}

	// Build rest.Config from kubeconfig; if file not present, try in-cluster
	var restCfg *rest.Config
	var err error
	if fi, errStat := os.Stat(kubeconfig); errStat == nil && !fi.IsDir() {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
		overrides := &clientcmd.ConfigOverrides{ClusterInfo: api.Cluster{}}
		if ca.Context != "" {
			overrides.CurrentContext = ca.Context
		}
		cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
		restCfg, err = cc.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("build rest config from kubeconfig: %w", err)
		}
	} else {
		// Fallback to in-cluster config
		restCfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("could not find kubeconfig at %q and in-cluster config failed: %w", kubeconfig, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Cluster{
		Config:   cfg,
		Rest:     restCfg,
		Client:   clientset,
		Context:  ca.Context,
		Kubeconf: kubeconfig,
	}, nil
}

// APIServerVersion returns the Kubernetes version string.
func (c *Cluster) APIServerVersion(ctx context.Context) (string, error) {
	v, err := c.Client.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return v.GitVersion, nil
}

// ListNamespaces lists namespaces, returning their names.
func (c *Cluster) ListNamespaces(ctx context.Context) ([]string, error) {
	ns, err := c.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ns.Items))
	for _, n := range ns.Items {
		names = append(names, n.Name)
	}
	return names, nil
}

// Ping performs a simple liveness check against the API (list namespaces with limit=1).
func (c *Cluster) Ping(ctx context.Context) error {
	_, err := c.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}
