package app

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

// DeployInput is the input for deploying an App's generated Kubernetes objects.
type DeployInput struct {
	// AppID identifies the target app to deploy.
	AppID string
}

// DeployOutput is the outcome of a deployment.
type DeployOutput struct {
	// Warnings contains non-fatal issues encountered during validation/conversion.
	Warnings []string
	// AppliedCount is the number of Kubernetes objects applied to the cluster.
	AppliedCount int
}

// Deploy validates and converts the app into Kubernetes objects and applies them to the target cluster.
func (u *UseCase) Deploy(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("DeployInput.AppID is required")
	}
	logger := logging.FromContext(ctx)

	// Resolve the target app for cluster/provider lookup.
	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil || appObj == nil {
		return nil, fmt.Errorf("failed to get app %s: %w", in.AppID, err)
	}

	// Reuse Validate to obtain Kubernetes objects and warnings.
	vout, err := u.Validate(ctx, &ValidateInput{AppID: in.AppID})
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if len(vout.Errors) > 0 {
		for _, e := range vout.Errors {
			logger.Error(ctx, e, "app", appObj.Name)
		}
		return nil, fmt.Errorf("validation failed (%d errors)", len(vout.Errors))
	}
	for _, w := range vout.Warnings {
		logger.Warn(ctx, w, "app", appObj.Name)
	}
	if len(vout.K8sObjects) == 0 {
		return nil, fmt.Errorf("no Kubernetes objects generated for app %s", appObj.Name)
	}

	// Resolve related cluster/provider/service for kubeconfig.
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

	// Instantiate provider driver and obtain kubeconfig.
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

	// Ensure GVK on objects for server-side apply.
	scheme := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(netv1.AddToScheme(scheme))
	for _, obj := range vout.K8sObjects {
		if obj == nil {
			continue
		}
		if gvk, _, err := scheme.ObjectKinds(obj); err == nil && len(gvk) > 0 {
			obj.GetObjectKind().SetGroupVersionKind(gvk[0])
		}
	}

	// Apply via server-side apply.
	if err := kcli.ApplyObjects(ctx, vout.K8sObjects, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
		return nil, fmt.Errorf("apply objects failed: %w", err)
	}

	logger.Info(ctx, "deploy success", "app", appObj.Name)
	return &DeployOutput{Warnings: vout.Warnings, AppliedCount: len(vout.K8sObjects)}, nil
}
