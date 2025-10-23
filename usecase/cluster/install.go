package cluster

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// InstallInput represents a command to install cluster resources.
type InstallInput struct {
	ClusterID string `json:"cluster_id"`
	Force     bool   `json:"force,omitempty"`
}
type InstallOutput struct{}

// Install installs in-cluster resources (Ingress Controller, etc.).
func (u *UseCase) Install(ctx context.Context, in *InstallInput) (*InstallOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}

	// Check protection policy
	// First-time install is detected by checking if installation is not yet done
	status, statusErr := u.ClusterPort.Status(ctx, c)
	isFirstTime := statusErr != nil || status == nil || !status.Installed

	opType := model.OpUpdate
	if isFirstTime {
		opType = model.OpCreate
	}
	if err := c.CheckInstallationProtection(opType); err != nil {
		return nil, err
	}

	var opts []model.ClusterInstallOption
	if in.Force {
		opts = append(opts, model.WithClusterInstallForce())
	}
	if err := u.ClusterPort.Install(ctx, c, opts...); err != nil {
		return nil, err
	}
	return &InstallOutput{}, nil
}
