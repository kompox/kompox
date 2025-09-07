package app

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogsInput defines parameters for fetching pod logs of an app.
type LogsInput struct {
	AppID     string `json:"app_id"`
	Container string `json:"container"`
	Follow    bool   `json:"follow"`
	TailLines *int64 `json:"tail_lines,omitempty"`
	// SinceTime is intentionally omitted for simplicity now; can be added later.
}

// LogsOutput currently does not carry structured data; logs are written to stdout/stderr directly.
// Reserved for future extension (e.g., returning last timestamp, truncated flags, etc.).
type LogsOutput struct{}

// Logs prints or streams logs from one selected pod of the app namespace.
// Strategy mirrors Exec(): pick a Ready, non tool-runner pod when possible.
func (u *UseCase) Logs(ctx context.Context, in *LogsInput) (*LogsOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("LogsInput.AppID is required")
	}

	// Resolve objects
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

	// Determine namespace via converter
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, "app")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	ns := c.Namespace

	// List pods
	podsList, err := kcli.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil || len(podsList.Items) == 0 {
		return nil, fmt.Errorf("app pod not found")
	}
	podName := ""
	for i := range podsList.Items {
		p := podsList.Items[i]
		if p.DeletionTimestamp != nil || p.Labels["kompox.dev/tool-runner"] == "true" {
			continue
		}
		ready := false
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				ready = true
				break
			}
		}
		if ready || podName == "" {
			podName = p.Name
		}
	}
	if podName == "" { // fallback
		podName = podsList.Items[0].Name
	}

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
