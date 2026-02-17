package main

import (
	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/usecase/app"
	"github.com/kompox/kompox/usecase/box"
	"github.com/kompox/kompox/usecase/cluster"
	"github.com/kompox/kompox/usecase/dns"
	"github.com/kompox/kompox/usecase/nodepool"
	"github.com/kompox/kompox/usecase/provider"
	"github.com/kompox/kompox/usecase/secret"
	"github.com/kompox/kompox/usecase/volume"
	"github.com/kompox/kompox/usecase/workspace"
	"github.com/spf13/cobra"
)

// buildAppUseCase creates app use case with required repositories.
func buildAppUseCase(cmd *cobra.Command) (*app.UseCase, error) {
	repos, err := buildAppRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &app.UseCase{
		Repos:      repos,
		VolumePort: providerdrv.GetVolumePort(repos.Workspace, repos.Provider, repos.Cluster, repos.App),
	}, nil
}

// buildClusterUseCase creates cluster use case with required repositories and ports.
func buildClusterUseCase(cmd *cobra.Command) (*cluster.UseCase, error) {
	repos, err := buildClusterRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &cluster.UseCase{
		Repos:       repos,
		ClusterPort: providerdrv.GetClusterPort(repos.Workspace, repos.Provider),
	}, nil
}

// buildProviderUseCase creates provider use case with required repositories.
func buildProviderUseCase(cmd *cobra.Command) (*provider.UseCase, error) {
	repos, err := buildProviderRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &provider.UseCase{Repos: repos}, nil
}

// buildWorkspaceUseCase creates workspace use case with required repositories.
func buildWorkspaceUseCase(cmd *cobra.Command) (*workspace.UseCase, error) {
	repos, err := buildWorkspaceRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &workspace.UseCase{Repos: repos}, nil
}

// buildVolumeUseCase creates volume use case with required repositories and ports.
func buildVolumeUseCase(cmd *cobra.Command) (*volume.UseCase, error) {
	repos, err := buildVolumeRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &volume.UseCase{
		Repos:      repos,
		VolumePort: providerdrv.GetVolumePort(repos.Workspace, repos.Provider, repos.Cluster, repos.App),
	}, nil
}

// buildBoxUseCase creates box use case with required repositories and ports.
func buildBoxUseCase(cmd *cobra.Command) (*box.UseCase, error) {
	repos, err := buildBoxRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &box.UseCase{
		Repos:      repos,
		VolumePort: providerdrv.GetVolumePort(repos.Workspace, repos.Provider, repos.Cluster, repos.App),
	}, nil
}

// buildSecretUseCase creates secret use case with required repositories.
func buildSecretUseCase(cmd *cobra.Command) (*secret.UseCase, error) {
	repos, err := buildSecretRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &secret.UseCase{Repos: repos}, nil
}

// buildDNSUseCase creates DNS use case with required repositories and ports.
func buildDNSUseCase(cmd *cobra.Command) (*dns.UseCase, error) {
	repos, err := buildDNSRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &dns.UseCase{
		Repos:       repos,
		ClusterPort: providerdrv.GetClusterPort(repos.Workspace, repos.Provider),
	}, nil
}

// buildNodePoolUseCase creates node pool use case with required repositories and ports.
func buildNodePoolUseCase(cmd *cobra.Command) (*nodepool.UseCase, error) {
	repos, err := buildNodePoolRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &nodepool.UseCase{
		Repos:        repos,
		NodePoolPort: providerdrv.GetNodePoolPort(repos.Workspace, repos.Provider),
	}, nil
}
