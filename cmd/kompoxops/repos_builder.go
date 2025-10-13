package main

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/usecase/app"
	"github.com/kompox/kompox/usecase/box"
	"github.com/kompox/kompox/usecase/cluster"
	"github.com/kompox/kompox/usecase/dns"
	"github.com/kompox/kompox/usecase/provider"
	"github.com/kompox/kompox/usecase/secret"
	"github.com/kompox/kompox/usecase/volume"
	"github.com/kompox/kompox/usecase/workspace"
	"github.com/spf13/cobra"
)

// buildRepos creates repositories based on db-url or CRD mode.
// If CRD mode is enabled, it uses the CRD sink instead of db-url.
func buildRepos(cmd *cobra.Command) (*domain.Repositories, error) {
	// If CRD mode is enabled, use CRD sink
	if crdMode.enabled && crdMode.sink != nil {
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()
		return buildReposFromCRD(ctx, crdMode.sink)
	}

	// Otherwise, use db-url
	return buildReposFromDB(cmd)
}

// buildAppRepos creates repositories needed for app use cases.
func buildAppRepos(cmd *cobra.Command) (*app.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &app.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
		App:       repos.App,
	}, nil
}

// buildClusterRepos creates repositories needed for cluster use cases.
func buildClusterRepos(cmd *cobra.Command) (*cluster.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &cluster.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
	}, nil
}

// buildProviderRepos creates repositories needed for provider use cases.
func buildProviderRepos(cmd *cobra.Command) (*provider.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &provider.Repos{
		Provider: repos.Provider,
	}, nil
}

// buildWorkspaceRepos creates repositories needed for workspace use cases.
func buildWorkspaceRepos(cmd *cobra.Command) (*workspace.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &workspace.Repos{
		Workspace: repos.Workspace,
	}, nil
}

// buildVolumeRepos creates repositories needed for volume use cases.
func buildVolumeRepos(cmd *cobra.Command) (*volume.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &volume.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
		App:       repos.App,
	}, nil
}

// buildBoxRepos creates repositories needed for box use cases.
func buildBoxRepos(cmd *cobra.Command) (*box.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &box.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
		App:       repos.App,
	}, nil
}

// buildSecretRepos creates repositories needed for secret use cases.
func buildSecretRepos(cmd *cobra.Command) (*secret.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &secret.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
		App:       repos.App,
	}, nil
}

// buildDNSRepos creates repositories needed for DNS use cases.
func buildDNSRepos(cmd *cobra.Command) (*dns.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &dns.Repos{
		Workspace: repos.Workspace,
		Provider:  repos.Provider,
		Cluster:   repos.Cluster,
		App:       repos.App,
	}, nil
}
