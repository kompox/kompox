package cluster

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// UpdateInput specifies cluster fields that can be modified.
type UpdateInput struct {
	// ClusterID identifies the cluster.
	ClusterID string `json:"cluster_id"`
	// Name optionally updates the cluster name.
	Name *string `json:"name,omitempty"`
	// ProviderID optionally updates the provider reference.
	ProviderID *string `json:"provider_id,omitempty"`
}

// UpdateOutput wraps the updated cluster.
type UpdateOutput struct {
	// Cluster is the updated entity.
	Cluster *model.Cluster `json:"cluster"`
}

// Update applies modifications to a cluster.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	existing, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	changed := false
	if in.Name != nil && *in.Name != "" && existing.Name != *in.Name {
		existing.Name = *in.Name
		changed = true
	}
	if in.ProviderID != nil && existing.ProviderID != *in.ProviderID {
		existing.ProviderID = *in.ProviderID
		changed = true
	}
	if changed {
		existing.UpdatedAt = time.Now().UTC()
		if err := u.Repos.Cluster.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	return &UpdateOutput{Cluster: existing}, nil
}
