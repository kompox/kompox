package provider

import "context"

type DeleteCommand struct{ ID string }

func (u *UseCase) Delete(ctx context.Context, cmd DeleteCommand) error {
	if cmd.ID == "" {
		return nil
	}
	return u.Providers.Delete(ctx, cmd.ID)
}
