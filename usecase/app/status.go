package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/internal/naming"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// StatusInput represents a command to get app status.
type StatusInput struct {
	// AppID identifies the app.
	AppID string `json:"app_id"`
}

// StatusOutput represents the response of app status.
// For now it focuses on the generated ingress hostnames like cluster status does for cluster fields.
type StatusOutput struct {
	AppID       string `json:"app_id"`
	AppName     string `json:"app_name"`
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	// Namespace is the actual Kubernetes namespace where the app resources reside (if found).
	Namespace string `json:"namespace,omitempty"`
	// Deployed indicates whether the app namespace exists in the cluster.
	Deployed bool `json:"deployed"`
	// IngressHosts is a combined unique list of hosts from custom and default-domain ingresses.
	IngressHosts []string `json:"ingress_hosts,omitempty"`
}

// Status returns status information about an app, including generated ingress hostnames.
func (u *UseCase) Status(ctx context.Context, in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("missing app ID")
	}

	// Fetch app and related resources
	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	if appObj == nil {
		return nil, fmt.Errorf("app not found: %s", in.AppID)
	}
	cls, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cls == nil {
		return nil, fmt.Errorf("cluster not found: %s", appObj.ClusterID)
	}
	prv, err := u.Repos.Provider.Get(ctx, cls.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}
	if prv == nil {
		return nil, fmt.Errorf("provider not found: %s", cls.ProviderID)
	}
	svc, err := u.Repos.Service.Get(ctx, prv.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// Compute expected namespace and find actual target namespace by labels
	hashes := naming.NewHashes(svc.Name, prv.Name, cls.Name, appObj.Name)
	expectedNS := hashes.Namespace

	// Build provider driver and kube client
	factory, ok := providerdrv.GetDriverFactory(prv.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", prv.Driver)
	}
	drv, err := factory(svc, prv)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", prv.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, cls)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Only accept the namespace whose name exactly matches expectedNS,
	// and only when it has the expected labels.
	nsName := ""
	ns, err := kcli.Clientset.CoreV1().Namespaces().Get(ctx, expectedNS, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get namespace %s: %w", expectedNS, err)
		}
	} else {
		// Verify required labels are present and match the expected values.
		want := labels.Set(map[string]string{
			"app.kubernetes.io/instance":   fmt.Sprintf("%s-%s", appObj.Name, hashes.AppInstance),
			"app.kubernetes.io/managed-by": "kompox",
		})
		has := labels.Set(ns.Labels)
		ok := true
		for k, v := range want {
			if has.Get(k) != v {
				ok = false
				break
			}
		}
		if ok {
			nsName = expectedNS
		}
	}

	// Determine if deployed by checking if any Deployment exists in the target namespace
	deployed := false
	if nsName != "" {
		if deps, err := kcli.Clientset.AppsV1().Deployments(nsName).List(ctx, metav1.ListOptions{}); err == nil {
			if len(deps.Items) > 0 {
				deployed = true
			}
		} else if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to list deployments in namespace %s: %w", nsName, err)
		}
	}

	// List all Ingress resources in the namespace and collect hostnames
	hostsSet := map[string]struct{}{}
	if nsName != "" {
		ings, err := kcli.Clientset.NetworkingV1().Ingresses(nsName).List(ctx, metav1.ListOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to list ingresses in namespace %s: %w", nsName, err)
			}
			// treat as empty
		} else {
			for _, ing := range ings.Items {
				for _, rule := range ing.Spec.Rules {
					if rule.Host != "" {
						hostsSet[rule.Host] = struct{}{}
					}
				}
			}
		}
	}
	var hosts []string
	for h := range hostsSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)

	out := &StatusOutput{
		AppID:        appObj.ID,
		AppName:      appObj.Name,
		ClusterID:    cls.ID,
		ClusterName:  cls.Name,
		Namespace:    nsName,
		Deployed:     deployed,
		IngressHosts: hosts,
	}
	// keep types stable (no-op use): ensure JSON tags compile
	_, _ = json.Marshal(out)
	return out, nil
}
