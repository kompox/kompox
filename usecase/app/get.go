package app

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// GetInput provides the identifier needed to fetch an App.
type GetInput struct {
	// AppID is the ID of the target application.
	AppID string `json:"app_id"`
}

// GetOutput wraps the retrieved App.
type GetOutput struct {
	// App is the fetched application entity.
	App *model.App `json:"app"`
}

// Get returns the App identified by AppID.
func (u *UseCase) Get(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, model.ErrAppInvalid
	}
	a, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	return &GetOutput{App: a}, nil
}
