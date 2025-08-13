package cluster

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

type UseCase struct{ Clusters domain.ClusterRepository }

type CreateCommand struct {
	Name       string
	ProviderID string
}

func (u *UseCase) Create(ctx context.Context, cmd CreateCommand) (*model.Cluster, error) {
	if cmd.Name == "" {
		return nil, model.ErrClusterInvalid
	}
	now := time.Now().UTC()
	c := &model.Cluster{ID: "", Name: cmd.Name, ProviderID: cmd.ProviderID, CreatedAt: now, UpdatedAt: now}
	if err := u.Clusters.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
