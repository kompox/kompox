package app

import "context"

type DeleteInput struct{ ID string }

func (u *UseCase) Delete(ctx context.Context, cmd DeleteInput) error {
	if cmd.ID == "" {
		return nil
	}
	return u.Apps.Delete(ctx, cmd.ID)
}
