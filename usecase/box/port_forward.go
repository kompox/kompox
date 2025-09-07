package box

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// setupPortForward sets up port forwarding to the specified port in the Kompox Box pod
func (u *UseCase) setupPortForward(ctx context.Context, appID string, remotePort int, localPort int) (int, func(), error) {
	logger := logging.FromContext(ctx)

	// Resolve environment dependencies
	appObj, err := u.Repos.App.Get(ctx, appID)
	if err != nil || appObj == nil {
		return 0, nil, fmt.Errorf("failed to get app %s: %w", appID, err)
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return 0, nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := u.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return 0, nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	// Create provider driver to get kubeconfig
	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return 0, nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Get namespace using converter
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, "box")
	if _, err := c.Convert(ctx); err != nil {
		return 0, nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.Namespace

	logger.Debug(ctx, "setting up port forward",
		"namespace", ns,
		"app_name", appObj.Name,
		"cluster_name", clusterObj.Name,
		"remote_port", remotePort)

	// Find the Kompox Box pod
	pod, err := kcli.FindPodByLabels(ctx, ns, LabelBoxSelector)
	if err != nil {
		return 0, nil, fmt.Errorf("find Kompox Box pod: %w", err)
	}

	logger.Debug(ctx, "found kompox box pod",
		"pod_name", pod.Name,
		"pod_phase", pod.Status.Phase,
		"pod_node", pod.Spec.NodeName)

	// Set up port forwarding to the specified port
	result, err := kcli.PortForward(ctx, &kube.PortForwardOptions{
		Namespace:  ns,
		PodName:    pod.Name,
		LocalPort:  localPort,  // specified local port
		RemotePort: remotePort, // specified remote port
	})
	if err != nil {
		return 0, nil, fmt.Errorf("setup port forward: %w", err)
	}

	logger.Debug(ctx, "port forward established",
		"local_port", result.LocalPort,
		"remote_port", remotePort,
		"pod_name", pod.Name,
		"namespace", ns)

	return result.LocalPort, result.StopFunc, nil
}
