package main

import (
	"github.com/spf13/cobra"
	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/usecase/app"
	"github.com/yaegashi/kompoxops/usecase/cluster"
	"github.com/yaegashi/kompoxops/usecase/provider"
	"github.com/yaegashi/kompoxops/usecase/service"
)

// buildAppUseCase creates app use case with required repositories.
func buildAppUseCase(cmd *cobra.Command) (*app.UseCase, error) {
	repos, err := buildAppRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &app.UseCase{Repos: repos}, nil
}

// buildClusterUseCase creates cluster use case with required repositories and ports.
func buildClusterUseCase(cmd *cobra.Command) (*cluster.UseCase, error) {
	repos, err := buildClusterRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &cluster.UseCase{
		Repos:       repos,
		ClusterPort: providerdrv.GetClusterPort(repos.Provider),
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
