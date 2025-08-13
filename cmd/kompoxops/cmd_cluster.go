package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/domain/model"
	uc "github.com/yaegashi/kompoxops/usecase/cluster"
	puc "github.com/yaegashi/kompoxops/usecase/provider"
)

func newCmdCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newCmdClusterCreate(),
		newCmdClusterDelete(),
		newCmdClusterProvision(),
		newCmdClusterDeprovision(),
		newCmdClusterInstall(),
		newCmdClusterUninstall(),
	)
	return cmd
}

func buildClusterUseCases(cmd *cobra.Command) (*uc.UseCase, *puc.UseCase, error) {
	_, providerRepo, clusterRepo, _, err := buildRepositories(cmd)
	if err != nil {
		return nil, nil, err
	}
	clusterUC := &uc.UseCase{Clusters: clusterRepo}
	providerUC := &puc.UseCase{Providers: providerRepo}
	return clusterUC, providerUC, nil
}

func newCmdClusterCreate() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a cluster resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cluster create command not implemented yet")
		},
	}
}

func newCmdClusterDelete() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete a cluster resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cluster delete command not implemented yet")
		},
	}
}

func newCmdClusterProvision() *cobra.Command {
	return &cobra.Command{
		Use:   "provision <cluster-name>",
		Short: "Provision a Kubernetes cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, providerUC, err := buildClusterUseCases(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
			defer cancel()

			// Get cluster by name
			clusterName := args[0]
			clusters, err := clusterUC.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			var cluster *model.Cluster
			for _, c := range clusters {
				if c.Name == clusterName {
					cluster = c
					break
				}
			}
			if cluster == nil {
				return fmt.Errorf("cluster %s not found", clusterName)
			}

			// Check if cluster is existing
			if cluster.Existing {
				fmt.Printf("Cluster %s is marked as existing, skipping provision\n", clusterName)
				return nil
			}

			// Get provider
			provider, err := providerUC.Get(ctx, cluster.ProviderID)
			if err != nil {
				return fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
			}

			// Get driver factory from registry
			factory, exists := getDriverFactory(provider.Driver)
			if !exists {
				return fmt.Errorf("unknown provider driver: %s", provider.Driver)
			}

			// Create driver instance
			driver, err := factory(provider.Settings)
			if err != nil {
				return fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
			}

			fmt.Printf("Provisioning cluster %s using provider %s (%s)...\n",
				clusterName, provider.Name, provider.Driver)

			// Provision the cluster
			if err := driver.ClusterProvision(cluster); err != nil {
				return fmt.Errorf("failed to provision cluster %s: %w", clusterName, err)
			}

			fmt.Printf("Successfully provisioned cluster %s\n", clusterName)
			return nil
		},
	}
}

func newCmdClusterDeprovision() *cobra.Command {
	return &cobra.Command{
		Use:   "deprovision <cluster-name>",
		Short: "Deprovision a Kubernetes cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, providerUC, err := buildClusterUseCases(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
			defer cancel()

			// Get cluster by name
			clusterName := args[0]
			clusters, err := clusterUC.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			var cluster *model.Cluster
			for _, c := range clusters {
				if c.Name == clusterName {
					cluster = c
					break
				}
			}
			if cluster == nil {
				return fmt.Errorf("cluster %s not found", clusterName)
			}

			// Check if cluster is existing
			if cluster.Existing {
				fmt.Printf("Cluster %s is marked as existing, skipping deprovision\n", clusterName)
				return nil
			}

			// Get provider
			provider, err := providerUC.Get(ctx, cluster.ProviderID)
			if err != nil {
				return fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
			}

			// Get driver factory from registry
			factory, exists := getDriverFactory(provider.Driver)
			if !exists {
				return fmt.Errorf("unknown provider driver: %s", provider.Driver)
			}

			// Create driver instance
			driver, err := factory(provider.Settings)
			if err != nil {
				return fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
			}

			fmt.Printf("Deprovisioning cluster %s using provider %s (%s)...\n",
				clusterName, provider.Name, provider.Driver)

			// Deprovision the cluster
			if err := driver.ClusterDeprovision(cluster); err != nil {
				return fmt.Errorf("failed to deprovision cluster %s: %w", clusterName, err)
			}

			fmt.Printf("Successfully deprovisioned cluster %s\n", clusterName)
			return nil
		},
	}
}

func newCmdClusterInstall() *cobra.Command {
	return &cobra.Command{
		Use:   "install <cluster-name>",
		Short: "Install cluster resources (Ingress Controller, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cluster install command not implemented yet")
		},
	}
}

func newCmdClusterUninstall() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <cluster-name>",
		Short: "Uninstall cluster resources (Ingress Controller, etc.)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cluster uninstall command not implemented yet")
		},
	}
}

// getDriverFactory is a helper function to access the driver registry
func getDriverFactory(driverName string) (func(map[string]string) (providerdrv.Driver, error), bool) {
	// This function needs access to the internal registry
	// We'll implement this by exposing a function in the registry package
	return providerdrv.GetDriverFactory(driverName)
}
