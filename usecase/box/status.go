package box

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusInput struct {
	AppID string `json:"app_id"`
}

type StatusOutput struct {
	Ready      bool     `json:"ready"`
	Image      string   `json:"image"`
	Namespace  string   `json:"namespace"`
	Node       string   `json:"node"`
	Deployment string   `json:"deployment"`
	Pod        string   `json:"pod"`
	Container  string   `json:"container"`
	Command    []string `json:"command"`
	Args       []string `json:"args"`
}

func (u *UseCase) Status(ctx context.Context, in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("StatusInput.AppID is required")
	}
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
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, "box")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.Namespace
	name := BoxResourceName

	dep, err := kcli.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &StatusOutput{
			Ready:      false,
			Image:      "",
			Namespace:  ns,
			Node:       "",
			Deployment: name,
			Pod:        "",
			Container:  BoxContainerName,
			Command:    nil,
			Args:       nil,
		}, nil
	}
	ready := dep.Status.ReadyReplicas >= 1
	var image string
	var command, args []string
	// find container spec by BoxContainerName
	for _, ct := range dep.Spec.Template.Spec.Containers {
		if ct.Name == BoxContainerName {
			image = ct.Image
			if len(ct.Command) > 0 {
				command = append([]string(nil), ct.Command...)
			}
			if len(ct.Args) > 0 {
				args = append([]string(nil), ct.Args...)
			}
			break
		}
	}

	pods, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: c.SelectorString})
	node := ""
	podName := ""
	if err == nil {
		for i := range pods.Items {
			p := pods.Items[i]
			if p.DeletionTimestamp != nil {
				continue
			}
			// try to find the container and node from the first non-deleting pod
			podName = p.Name
			node = p.Spec.NodeName
			break
		}
	}

	return &StatusOutput{
		Ready:      ready,
		Image:      image,
		Namespace:  ns,
		Node:       node,
		Deployment: name,
		Pod:        podName,
		Container:  BoxContainerName,
		Command:    command,
		Args:       args,
	}, nil
}
