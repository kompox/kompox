package cluster

import "context"

type DeleteCommand struct{ ID string }

func (u *UseCase) Delete(ctx context.Context, cmd DeleteCommand) error {
	if cmd.ID == "" {
		return nil
	}
	return u.Clusters.Delete(ctx, cmd.ID)
}
