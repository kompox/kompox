package service

import (
	"context"
)

// DeleteInput identifies the service to delete.
type DeleteInput struct {
	// ServiceID is the service identifier.
	ServiceID string `json:"service_id"`
}

// DeleteOutput is empty because delete has no return entity.
type DeleteOutput struct{}

// Delete removes a service; empty ID is a no-op.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.ServiceID == "" { // idempotent no-op
		return &DeleteOutput{}, nil
	}
	if err := u.Repos.Service.Delete(ctx, in.ServiceID); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
