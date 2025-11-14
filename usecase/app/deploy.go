package app

import (
	"context"
	"fmt"
	"sort"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/internal/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	msgSym := "UC:app.deploy"

	// Resolve the target app for cluster/provider lookup.
	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil || appObj == nil {
		return nil, fmt.Errorf("failed to get app %s: %w", in.AppID, err)
	}

	res, err := u.validateApp(ctx, appObj)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if res == nil {
		return nil, fmt.Errorf("validation result unavailable")
	}
	_, warnCount, errCount := countIssuesBySeverity(res.Issues)
	for _, issue := range res.Issues {
		switch issue.Severity {
		case SeverityError:
			logger.Error(ctx, msgSym+":ValidationError", "code", issue.Code, "desc", issue.Message)
		case SeverityWarn:
			logger.Warn(ctx, msgSym+":ValidationWarning", "code", issue.Code, "desc", issue.Message)
		default:
			logger.Info(ctx, msgSym+":ValidationInfo", "code", issue.Code, "desc", issue.Message)
		}
	}
	if warnCount > 0 || errCount > 0 {
		return nil, fmt.Errorf("validation blocked (%d warnings, %d errors)", warnCount, errCount)
	}
	if len(res.K8sObjects) == 0 {
		return nil, fmt.Errorf("no Kubernetes objects generated for app %s", appObj.Name)
	}

	// Resolve related cluster/provider/workspace for kubeconfig.
	clusterObj := res.cluster
	if clusterObj == nil {
		if clusterObj, err = u.Repos.Cluster.Get(ctx, appObj.ClusterID); err != nil || clusterObj == nil {
			return nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
		}
	}
	providerObj := res.provider
	if providerObj == nil {
		if providerObj, err = u.Repos.Provider.Get(ctx, clusterObj.ProviderID); err != nil || providerObj == nil {
			return nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
		}
	}
	workspaceObj := res.workspace
	if workspaceObj == nil && providerObj.WorkspaceID != "" {
		workspaceObj, _ = u.Repos.Workspace.Get(ctx, providerObj.WorkspaceID)
	}

	// Instantiate provider driver and obtain kubeconfig.
	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(workspaceObj, providerObj)
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
	for _, obj := range res.K8sObjects {
		if obj == nil {
			continue
		}
		if gvk, _, err := scheme.ObjectKinds(obj); err == nil && len(gvk) > 0 {
			obj.GetObjectKind().SetGroupVersionKind(gvk[0])
		}
	}

	// Apply via server-side apply.
	if err := kcli.ApplyObjects(ctx, res.K8sObjects, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
		return nil, fmt.Errorf("apply objects failed: %w", err)
	}

	// Inline prune of stale headless Services using converter metadata.
	if res.Converter != nil {
		// Desired names from converter (authoritative)
		desired := map[string]struct{}{}
		for _, hs := range res.Converter.K8sHeadlessServices {
			if hs != nil {
				desired[hs.Name] = struct{}{}
			}
		}
		// List current headless services via selector (app + marker)
		selector := res.Converter.HeadlessServiceSelectorString
		// Namespace: derive from converter namespace (avoid assuming K8sObjects[0])
		ns := res.Converter.Namespace
		listLogger := logging.FromContext(ctx).With("ns", ns, "selector", selector)
		list, listErr := kcli.Clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if listErr != nil {
			listLogger.Info(ctx, msgSym+":HeadlessServices:List/efail", "err", listErr)
		} else {
			listLogger.Info(ctx, msgSym+":HeadlessServices:List/eok")
			var toDelete []string
			for _, svc := range list.Items {
				if _, ok := desired[svc.Name]; ok {
					continue
				}
				toDelete = append(toDelete, svc.Name)
			}
			sort.Strings(toDelete)
			for _, name := range toDelete {
				nameLogger := logger.With("ns", ns, "name", name)
				if derr := kcli.Clientset.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{}); derr != nil {
					nameLogger.Info(ctx, msgSym+":HeadlessServices:Delete/efail", "err", derr)
				} else {
					nameLogger.Info(ctx, msgSym+":HeadlessServices:Delete/eok")
				}
			}
		}
	}

	// Runtime patch: recompute PodContentHash via kube.Client method.
	if res.Converter != nil && res.Converter.K8sDeployment != nil {
		depName := res.Converter.K8sDeployment.Name
		depNS := res.Converter.K8sDeployment.Namespace
		if err := kcli.PatchDeploymentPodContentHash(ctx, depNS, depName); err != nil {
			logger.Warn(ctx, msgSym+":PatchFailed", "err", err)
		}
	}

	return &DeployOutput{Warnings: issueMessagesBySeverity(res.Issues, SeverityInfo), AppliedCount: len(res.K8sObjects)}, nil
}
