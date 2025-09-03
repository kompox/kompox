package cluster

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogsInput defines parameters for fetching traefik pod logs of a cluster.
type LogsInput struct {
	ClusterID string `json:"cluster_id"`
	Container string `json:"container"`
	Follow    bool   `json:"follow"`
	TailLines *int64 `json:"tail_lines,omitempty"`
}

type LogsOutput struct{}

// Logs fetches logs from a traefik ingress pod in the cluster.
func (u *UseCase) Logs(ctx context.Context, in *LogsInput) (*LogsOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, fmt.Errorf("LogsInput.ClusterID is required")
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil || clusterObj == nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", in.ClusterID, err)
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
	// traefik ingress pod: namespace is determined by kube.IngressNamespace(clusterObj)
	ns := kube.IngressNamespace(clusterObj)
	pods, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=traefik",
	})
	if err != nil || len(pods.Items) == 0 {
		return nil, fmt.Errorf("traefik pod not found in namespace %s", ns)
	}
	podName := pods.Items[0].Name

	_, err = kcli.PodLog(ctx, &kube.PodLogInput{
		Namespace: ns,
		Pod:       podName,
		Container: in.Container,
		Follow:    in.Follow,
		TailLines: in.TailLines,
	})
	if err != nil {
		return nil, err
	}
	return &LogsOutput{}, nil
}
