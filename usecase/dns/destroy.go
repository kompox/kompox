package dns

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
)

// DestroyInput holds parameters for DNS record destruction.
type DestroyInput struct {
	AppID         string `json:"app_id"` // required: app to destroy DNS for
	ComponentName string `json:"component_name,omitempty"`
	Strict        bool   `json:"strict,omitempty"`
	DryRun        bool   `json:"dry_run,omitempty"`
}

// DestroyOutput holds the result of DNS record destruction.
type DestroyOutput struct {
	Deleted []DNSRecordResult `json:"deleted"`
}

// Destroy destroys DNS records for the cluster and its apps.
func (u *UseCase) Destroy(ctx context.Context, in *DestroyInput) (*DestroyOutput, error) {
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

	// Get provider and service
	provider, err := u.Repos.Provider.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	var service *model.Service
	if provider.ServiceID != "" {
		service, _ = u.Repos.Service.Get(ctx, provider.ServiceID)
	}

	// Create kube.Converter to get namespace and selector
	c := kube.NewConverter(service, provider, cluster, app, in.ComponentName)

	// Build provider driver to get kubeconfig
	factory, ok := providerdrv.GetDriverFactory(provider.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}
	drv, err := factory(service, provider)
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
		return &DestroyOutput{}, nil
	}

	// Build DNS apply options
	var opts []model.ClusterDNSApplyOption
	if in.Strict {
		opts = append(opts, model.WithClusterDNSApplyStrict())
	}
	if in.DryRun {
		opts = append(opts, model.WithClusterDNSApplyDryRun())
	}

	// Delete DNS records for each FQDN.
	// Attempt deletion for common record types (A, AAAA, CNAME).
	var results []DNSRecordResult
	recordTypes := []model.DNSRecordType{
		model.DNSRecordTypeA,
		model.DNSRecordTypeAAAA,
		model.DNSRecordTypeCNAME,
	}

	for _, host := range ingressHosts {
		for _, recordType := range recordTypes {
			// Build deletion record set (empty RData)
			rset := model.DNSRecordSet{
				FQDN:  host.Host,
				Type:  recordType,
				TTL:   0,
				RData: nil,
			}

			err := u.ClusterPort.DNSApply(ctx, cluster, rset, opts...)
			result := DNSRecordResult{
				FQDN: host.Host,
				Type: recordType,
			}

			if err != nil {
				result.Action = "failed"
				result.Message = err.Error()
				// Best effort: continue even on failure unless strict
				if in.Strict {
					return nil, fmt.Errorf("delete DNS for %s (%s): %w", host.Host, recordType, err)
				}
			} else {
				if in.DryRun {
					result.Action = "planned"
					result.Message = fmt.Sprintf("would delete %s record", recordType)
				} else {
					result.Action = "deleted"
					result.Message = fmt.Sprintf("%s record deleted", recordType)
				}
			}

			results = append(results, result)
		}
	}

	return &DestroyOutput{Deleted: results}, nil
}
