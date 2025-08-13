package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) List(ctx context.Context) ([]*model.Cluster, error) { return u.Clusters.List(ctx) }
