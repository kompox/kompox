package kube

import (
	"context"
	"fmt"
	"sort"

	"github.com/kompox/kompox/domain/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressHostIP holds hostname and IP address pair from Ingress resources.
type IngressHostIP struct {
	Host string // Hostname from Ingress rule
	IP   string // IP address from Ingress status (may be empty)
}

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

// IngressHostIPs returns all unique hostnames with their IP addresses from Ingress resources matching the label selector.
// It queries the specified namespace and collects hosts from Ingress.Spec.Rules[].Host along with IPs from Ingress.Status.LoadBalancer.Ingress[].
// Returns a sorted list of IngressHost. Empty list is returned if no matching Ingress resources are found.
// IP addresses may be empty if the Ingress has no LoadBalancer status yet.
func (c *Client) IngressHostIPs(ctx context.Context, namespace string, labelSelector string) ([]IngressHostIP, error) {
	if c == nil || c.Clientset == nil {
		return nil, fmt.Errorf("kube client is not initialized")
	}

	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	if labelSelector == "" {
		return nil, fmt.Errorf("labelSelector is required")
	}

	// List all Ingress resources matching the label selector in the specified namespace
	ings, err := c.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list ingresses with selector %q in namespace %q: %w", labelSelector, namespace, err)
	}

	// Collect unique hostnames with their IPs
	hostsMap := map[string]string{} // Host -> IP
	for _, ing := range ings.Items {
		// Get IP from LoadBalancer status (use first one if available)
		var ip string
		if len(ing.Status.LoadBalancer.Ingress) > 0 {
			ip = ing.Status.LoadBalancer.Ingress[0].IP
		}

		// Collect hostnames from rules
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				// Use IP from this Ingress's status
				// If multiple Ingresses have the same host, the last one wins
				hostsMap[rule.Host] = ip
			}
		}
	}

	// Convert to sorted slice
	hostIPs := make([]IngressHostIP, 0, len(hostsMap))
	for host, ip := range hostsMap {
		hostIPs = append(hostIPs, IngressHostIP{
			Host: host,
			IP:   ip,
		})
	}

	// Sort by Host
	sort.Slice(hostIPs, func(i, j int) bool {
		return hostIPs[i].Host < hostIPs[j].Host
	})

	return hostIPs, nil
}
