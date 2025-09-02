package tool

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DestroyInput contains parameters to destroy the maintenance runner resources.
type DestroyInput struct {
	// AppID is the target application id.
	AppID string `json:"app_id"`
}

// DestroyOutput is returned after destroying runner workload resources.
type DestroyOutput struct{}

// Destroy deletes the runner Deployment and related workload resources (not PV/PVC).
func (u *UseCase) Destroy(ctx context.Context, in *DestroyInput) (*DestroyOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("DestroyInput.AppID is required")
	}

	// Resolve app -> cluster -> provider/service
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

	// Driver and kube client
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

	// Determine namespace via converter
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}

	// Delete only the runner Deployment by label
	selector := "kompox.dev/tool-runner=true"
	targets := []kube.DeleteResourceTarget{
		{GVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, Namespaced: true, Kind: "Deployment"},
		{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, Namespaced: true, Kind: "Pod"}, // cleanup any stray pods
	}
	if _, err := kcli.DeleteByLabelSelector(ctx, c.NSName, targets, selector, &kube.DeleteBySelectorOptions{}); err != nil {
		return nil, fmt.Errorf("delete runner failed: %w", err)
	}
	return &DestroyOutput{}, nil
}
