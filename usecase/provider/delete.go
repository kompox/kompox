package provider

import "context"

// DeleteInput identifies the provider to remove.
type DeleteInput struct {
	// ProviderID is the target identifier.
	ProviderID string `json:"provider_id"`
}

// DeleteOutput is empty because delete has no return entity.
type DeleteOutput struct{}

// Delete removes a provider; empty ID is a no-op.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.ProviderID == "" { // idempotent
		return &DeleteOutput{}, nil
	}
	if err := u.Repos.Provider.Delete(ctx, in.ProviderID); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
