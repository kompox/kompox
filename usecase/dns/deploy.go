package dns

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
)

// DeployInput holds parameters for DNS record deployment.
type DeployInput struct {
	AppID         string `json:"app_id"` // required: app to deploy DNS for
	ComponentName string `json:"component_name,omitempty"`
	Strict        bool   `json:"strict,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
}

// DeployOutput holds the result of DNS record deployment.
type DeployOutput struct {
	Applied []DNSRecordResult `json:"applied"`
}

// DNSRecordResult describes the result of a DNS operation.
type DNSRecordResult struct {
	FQDN    string              `json:"fqdn"`
	Type    model.DNSRecordType `json:"type"`
	Action  string              `json:"action"` // "created", "updated", "deleted", "skipped", "failed", "planned"
	Message string              `json:"message"`
}

// Deploy deploys DNS records for the cluster and its apps.
func (u *UseCase) Deploy(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("input is nil")
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("AppID is required")
	}
	if in.ComponentName == "" {
		in.ComponentName = "app"
	}

	// Get app and cluster
	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, fmt.Errorf("get app: %w", err)
	}
	cluster, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}

	// Get provider and workspace
	provider, err := u.Repos.Provider.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	var workspace *model.Workspace
	if provider.WorkspaceID != "" {
		workspace, _ = u.Repos.Workspace.Get(ctx, provider.WorkspaceID)
	}

	// Create kube.Converter to get namespace and selector
	c := kube.NewConverter(workspace, provider, cluster, app, in.ComponentName)

	// Build provider driver to get kubeconfig
	factory, ok := providerdrv.GetDriverFactory(provider.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}
	drv, err := factory(workspace, provider)
	if err != nil {
		return nil, fmt.Errorf("create driver: %w", err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}

	// Create kube client
	client, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	// Get ingress hosts using kube.Client
	ingressHosts, err := client.IngressHostIPs(ctx, c.Namespace, c.SelectorString)
	if err != nil {
		return nil, fmt.Errorf("get ingress hosts: %w", err)
	}

	if len(ingressHosts) == 0 {
		return &DeployOutput{}, nil
	}

	// Build DNS apply options
	var opts []model.ClusterDNSApplyOption
	if in.Strict {
		opts = append(opts, model.WithClusterDNSApplyStrict())
	}
	if in.DryRun {
		opts = append(opts, model.WithClusterDNSApplyDryRun())
	}

	// Apply DNS records for each ingress host
	var results []DNSRecordResult
	for _, host := range ingressHosts {
		// Use IP from IngressHost if available, otherwise skip
		if host.IP == "" {
			// No IP available yet for this host
			if !in.Strict {
				results = append(results, DNSRecordResult{
					FQDN:    host.Host,
					Action:  "skipped",
					Message: "no IP address available yet",
				})
				continue
			}
			return nil, fmt.Errorf("no IP address available for %s", host.Host)
		}

		// Build DNS record set (A record with IP)
		rset := model.DNSRecordSet{
			FQDN:  host.Host,
			Type:  model.DNSRecordTypeA,
			TTL:   0, // Use provider default
			RData: []string{host.IP},
		}

		err := u.ClusterPort.DNSApply(ctx, cluster, rset, opts...)
		result := DNSRecordResult{
			FQDN: host.Host,
			Type: rset.Type,
		}

		if err != nil {
			result.Action = "failed"
			result.Message = err.Error()
			if in.Strict {
				return nil, fmt.Errorf("apply DNS for %s: %w", host.Host, err)
			}
		} else {
			if in.DryRun {
				result.Action = "planned"
				result.Message = fmt.Sprintf("would create/update %s -> %s", rset.Type, host.IP)
			} else {
				result.Action = "updated"
				result.Message = fmt.Sprintf("%s -> %s", rset.Type, host.IP)
			}
		}

		results = append(results, result)
	}

	return &DeployOutput{Applied: results}, nil
}
