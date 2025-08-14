package cluster

import "context"

type DeleteInput struct{ ID string }

func (u *UseCase) Delete(ctx context.Context, cmd DeleteInput) error {
	if cmd.ID == "" {
		return nil
	}
	return u.Clusters.Delete(ctx, cmd.ID)
}
