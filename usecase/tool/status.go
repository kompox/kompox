package tool

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatusInput contains parameters to get runner status.
type StatusInput struct {
	// AppID is the target application id.
	AppID string `json:"app_id"`
}

// StatusOutput returns basic status of the runner and its mounts.
type StatusOutput struct {
	// Ready indicates whether the runner Pod is ready.
	Ready bool `json:"ready"`
	// Namespace where the runner lives.
	Namespace string `json:"namespace"`
	// Name of the runner workload.
	Name string `json:"name"`
	// NodeName where the Pod is running (if any).
	NodeName string `json:"node_name"`
	// Image of the runner container.
	Image string `json:"image"`
	// Command configured for the runner container.
	Command []string `json:"command"`
	// Args configured for the runner container.
	Args []string `json:"args"`
}

// Status returns the current status of the maintenance runner for the App.
func (u *UseCase) Status(ctx context.Context, in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("StatusInput.AppID is required")
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
	name := "tool-runner"

	// Try get Deployment
	dep, err := kcli.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// If not found, report not ready
		return &StatusOutput{Ready: false, Namespace: ns, Name: name}, nil
	}
	ready := dep.Status.ReadyReplicas >= 1
	// Extract container spec
	var image string
	var command, args []string
	for _, c := range dep.Spec.Template.Spec.Containers {
		if c.Name == "runner" {
			image = c.Image
			if len(c.Command) > 0 {
				command = append([]string(nil), c.Command...)
			}
			if len(c.Args) > 0 {
				args = append([]string(nil), c.Args...)
			}
			break
		}
	}

	// Find a Pod with matching labels
	pods, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "kompox.dev/tool-runner=true"})
	node := ""
	if err == nil {
		for i := range pods.Items {
			p := pods.Items[i]
			if p.DeletionTimestamp != nil {
				continue
			}
			node = p.Spec.NodeName
			break
		}
	}
	return &StatusOutput{Ready: ready, Namespace: ns, Name: name, NodeName: node, Image: image, Command: command, Args: args}, nil
}
