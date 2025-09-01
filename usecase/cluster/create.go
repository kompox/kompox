package cluster

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// CreateInput contains data to create a cluster.
type CreateInput struct {
	// Name is the cluster name.
	Name string `json:"name"`
	// ProviderID references the provider.
	ProviderID string `json:"provider_id"`
}

// CreateOutput wraps the created cluster.
type CreateOutput struct {
	// Cluster is the new cluster entity.
	Cluster *model.Cluster `json:"cluster"`
}

// Create persists a new cluster.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.Name == "" {
		return nil, model.ErrClusterInvalid
	}
	now := time.Now().UTC()
	c := &model.Cluster{ID: "", Name: in.Name, ProviderID: in.ProviderID, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.Cluster.Create(ctx, c); err != nil {
		return nil, err
	}
	return &CreateOutput{Cluster: c}, nil
}
