package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/domain/model"
	uc "github.com/yaegashi/kompoxops/usecase/cluster"
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
		newCmdClusterStatus(),
	)
	return cmd
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
			clusterUC, err := buildClusterUseCase(cmd)
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

			fmt.Printf("Provisioning cluster %s...\n", clusterName)

			// Provision the cluster via usecase
			if err := clusterUC.Provision(ctx, uc.ProvisionInput{ID: cluster.ID}); err != nil {
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
			clusterUC, err := buildClusterUseCase(cmd)
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

			fmt.Printf("Deprovisioning cluster %s...\n", clusterName)

			// Deprovision the cluster via usecase
			if err := clusterUC.Deprovision(ctx, uc.DeprovisionInput{ID: cluster.ID}); err != nil {
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

func newCmdClusterStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status <cluster-name>",
		Short: "Show cluster status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
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

			// Get status
			status, err := clusterUC.Status(ctx, uc.StatusInput{ID: cluster.ID})
			if err != nil {
				return fmt.Errorf("failed to get cluster status: %w", err)
			}

			// Output status as JSON
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		},
	}
}
