package main

import (
	"github.com/spf13/cobra"
	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/usecase/app"
	"github.com/kompox/kompox/usecase/cluster"
	"github.com/kompox/kompox/usecase/provider"
	"github.com/kompox/kompox/usecase/service"
	"github.com/kompox/kompox/usecase/volume"
)

// buildAppUseCase creates app use case with required repositories.
func buildAppUseCase(cmd *cobra.Command) (*app.UseCase, error) {
	repos, err := buildAppRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &app.UseCase{Repos: repos, VolumePort: providerdrv.GetVolumePort(repos.Service, repos.Provider, repos.Cluster, repos.App)}, nil
}

// buildClusterUseCase creates cluster use case with required repositories and ports.
func buildClusterUseCase(cmd *cobra.Command) (*cluster.UseCase, error) {
	repos, err := buildClusterRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &cluster.UseCase{
		Repos:       repos,
		ClusterPort: providerdrv.GetClusterPort(repos.Service, repos.Provider),
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

// buildServiceUseCase creates service use case with required repositories.
func buildServiceUseCase(cmd *cobra.Command) (*service.UseCase, error) {
	repos, err := buildServiceRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &service.UseCase{Repos: repos}, nil
}

// buildVolumeUseCase creates volume use case with required repositories and ports.
func buildVolumeUseCase(cmd *cobra.Command) (*volume.UseCase, error) {
	repos, err := buildVolumeRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &volume.UseCase{
		Repos:      repos,
		VolumePort: providerdrv.GetVolumePort(repos.Service, repos.Provider, repos.Cluster, repos.App),
	}, nil
}
