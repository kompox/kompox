package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
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
type StatusOutput struct {
	AppID        string   `json:"app_id"`
	AppName      string   `json:"app_name"`
	ClusterID    string   `json:"cluster_id"`
	ClusterName  string   `json:"cluster_name"`
	Ready        bool     `json:"ready"`
	Image        string   `json:"image"`
	Namespace    string   `json:"namespace"`
	Node         string   `json:"node"`
	Deployment   string   `json:"deployment"`
	Pod          string   `json:"pod"`
	Container    string   `json:"container"`
	Command      []string `json:"command"`
	Args         []string `json:"args"`
	IngressHosts []string `json:"ingress_hosts"`
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

	// Compute namespace and selector via converter
	c := kube.NewConverter(svc, prv, cls, appObj, "app")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	expectedNS := c.Namespace

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
		// Verify required labels (ALL scope) are present and match the expected values.
		want := labels.Set(c.BaseLabels)
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

	// Determine if deployed and get deployment details
	ready := false
	var image string
	var command, args []string
	var deployment, pod, container, node string

	if nsName != "" {
		// Get deployment using converter selector
		deps, err := kcli.Clientset.AppsV1().Deployments(nsName).List(ctx, metav1.ListOptions{LabelSelector: c.SelectorString})
		if err == nil && len(deps.Items) > 0 {
			dep := deps.Items[0] // Use first deployment
			deployment = dep.Name
			ready = dep.Status.ReadyReplicas >= 1

			// Get container details from first container
			if len(dep.Spec.Template.Spec.Containers) > 0 {
				ct := dep.Spec.Template.Spec.Containers[0]
				container = ct.Name
				image = ct.Image
				if len(ct.Command) > 0 {
					command = append([]string(nil), ct.Command...)
				}
				if len(ct.Args) > 0 {
					args = append([]string(nil), ct.Args...)
				}
			}
		}

		// Get pod details
		pods, err := kcli.Clientset.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{LabelSelector: c.SelectorString})
		if err == nil {
			for i := range pods.Items {
				p := pods.Items[i]
				if p.DeletionTimestamp != nil {
					continue
				}
				// Use first non-deleting pod
				pod = p.Name
				node = p.Spec.NodeName
				break
			}
		}
	}

	// List all Ingress resources using converter selector and collect hostnames
	hostsSet := map[string]struct{}{}
	if nsName != "" {
		ings, err := kcli.Clientset.NetworkingV1().Ingresses(nsName).List(ctx, metav1.ListOptions{LabelSelector: c.SelectorString})
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
		Ready:        ready,
		Image:        image,
		Namespace:    nsName,
		Node:         node,
		Deployment:   deployment,
		Pod:          pod,
		Container:    container,
		Command:      command,
		Args:         args,
		IngressHosts: hosts,
	}
	// keep types stable (no-op use): ensure JSON tags compile
	_, _ = json.Marshal(out)
	return out, nil
}
