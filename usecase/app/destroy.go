package app

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DestroyInput specifies parameters to destroy app resources from its cluster.
type DestroyInput struct {
	// AppID identifies the target app to destroy.
	AppID string
	// DeleteNamespace when true deletes the namespace after object deletion.
	DeleteNamespace bool
}

// DestroyOutput reports the result of a destroy operation.
type DestroyOutput struct {
	// DeletedCount is the number of resources deleted.
	DeletedCount int
	// Namespace is the computed namespace name for the app.
	Namespace string
	// LabelSelector used for selecting resources to delete.
	LabelSelector string
}

// Destroy deletes resources labeled as managed by kompox for the target app, and optionally deletes the Namespace.
func (u *UseCase) Destroy(ctx context.Context, in *DestroyInput) (*DestroyOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("DestroyInput.AppID is required")
	}
	logger := logging.FromContext(ctx)

	// Resolve app and related resources.
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

	// Compute namespace and selector via converter.
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, "app")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	nsName := c.Namespace
	labelSelector := c.SelectorString

	// Provider driver for kubeconfig.
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

	logger.Info(ctx, "deleting resources", "namespace", nsName, "selector", labelSelector)
	deletedCount, _ := kcli.DeleteByLabelSelector(ctx, nsName, kube.DefaultAppDeleteTargets(), labelSelector, &kube.DeleteBySelectorOptions{
		Propagation:  metav1.DeletePropagationBackground,
		IgnoreErrors: true,
	})

	if in.DeleteNamespace {
		logger.Info(ctx, "deleting namespace", "namespace", nsName)
		if err := kcli.DeleteNamespace(ctx, nsName); err != nil {
			return nil, fmt.Errorf("delete namespace %s failed: %w", nsName, err)
		}
	}

	logger.Info(ctx, "destroy completed", "app", appObj.Name, "namespace", nsName, "deleted", deletedCount, "deleteNamespace", in.DeleteNamespace)
	return &DestroyOutput{DeletedCount: deletedCount, Namespace: nsName, LabelSelector: labelSelector}, nil
}
