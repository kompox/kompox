package kube

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressEndpoint returns the external IP and FQDN (if any) of the ingress Service.
// It looks up the LoadBalancer status of the Service in the resolved ingress namespace.
// When the Service or fields are not found, it returns empty strings without error.
func (c *Client) IngressEndpoint(ctx context.Context, cluster *model.Cluster) (string, string, error) {
	if c == nil || c.Clientset == nil {
		return "", "", fmt.Errorf("kube client is not initialized")
	}

	ns := IngressNamespace(cluster)
	svcName := IngressServiceName(cluster)

	svc, err := c.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("get service %s/%s: %w", ns, svcName, err)
	}

	if svc == nil || svc.Status.LoadBalancer.Ingress == nil || len(svc.Status.LoadBalancer.Ingress) == 0 {
		return "", "", nil
	}

	ing := svc.Status.LoadBalancer.Ingress[0]
	return ing.IP, ing.Hostname, nil
}
