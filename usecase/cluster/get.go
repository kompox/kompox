package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) Get(ctx context.Context, id string) (*model.Cluster, error) {
	if id == "" {
		return nil, model.ErrClusterInvalid
	}
	return u.Repos.Cluster.Get(ctx, id)
}
