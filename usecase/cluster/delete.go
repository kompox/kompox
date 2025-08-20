package cluster

import "context"

// DeleteInput identifies the cluster to remove.
type DeleteInput struct {
	// ClusterID is the identifier of the cluster to delete.
	ClusterID string `json:"cluster_id"`
}

// DeleteOutput is empty because delete has no payload.
type DeleteOutput struct{}

// Delete removes a cluster; empty ID is a no-op.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.ClusterID == "" { // idempotent
		return &DeleteOutput{}, nil
	}
	if err := u.Repos.Cluster.Delete(ctx, in.ClusterID); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
