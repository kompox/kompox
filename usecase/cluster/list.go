package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ListInput defines optional future filters for clusters.
type ListInput struct{}

// ListOutput wraps a collection of clusters.
type ListOutput struct {
	// Clusters is the returned set.
	Clusters []*model.Cluster `json:"clusters"`
}

// List returns all clusters.
func (u *UseCase) List(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	items, err := u.Repos.Cluster.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Clusters: items}, nil
}
