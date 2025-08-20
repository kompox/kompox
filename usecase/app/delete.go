package app

import "context"

// DeleteInput identifies the App to delete.
type DeleteInput struct {
	// AppID is the identifier of the application to delete.
	AppID string `json:"app_id"`
}

// DeleteOutput is empty; deletion has no return object.
type DeleteOutput struct{}

// Delete removes the specified App. A missing or empty AppID is treated as a no-op.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.AppID == "" { // idempotent
		return &DeleteOutput{}, nil
	}
	if err := u.Repos.App.Delete(ctx, in.AppID); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
