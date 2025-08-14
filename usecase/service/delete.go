package service

import (
	"context"
)

type DeleteInput struct {
	ID string
}

func (u *UseCase) Delete(ctx context.Context, cmd DeleteInput) error {
	if cmd.ID == "" {
		return nil // idempotent no-op
	}
	return u.Repos.Service.Delete(ctx, cmd.ID)
}
