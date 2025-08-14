package service

import (
	"context"
)

type DeleteServiceCommand struct {
	ID string
}

func (u *UseCase) Delete(ctx context.Context, cmd DeleteServiceCommand) error {
	if cmd.ID == "" {
		return nil // idempotent no-op
	}
	return u.Services.Delete(ctx, cmd.ID)
}
