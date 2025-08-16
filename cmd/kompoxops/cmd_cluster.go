package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"
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
		newCmdClusterProvision(),
		newCmdClusterDeprovision(),
		newCmdClusterInstall(),
		newCmdClusterUninstall(),
		newCmdClusterStatus(),
	)
	return cmd
}

// getClusterName returns the cluster name from args if present, otherwise from loaded configuration file.
func getClusterName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if configRoot != nil && len(configRoot.Cluster.Name) > 0 {
		return configRoot.Cluster.Name, nil
	}
	return "", fmt.Errorf("cluster name not specified and no default available; provide cluster-name or use --db-url=file:/path/to/kompoxops.yml")
}

func newCmdClusterProvision() *cobra.Command {
	return &cobra.Command{
		Use:   "provision [cluster-name]",
		Short: "Provision a Kubernetes cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)
			clusterName, err := getClusterName(cmd, args)
			if err != nil {
				return err
			}
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
			if cluster.Existing {
				logger.Info(ctx, "cluster marked as existing, skipping provision", "cluster", clusterName)
				return nil
			}

			logger.Info(ctx, "provision start", "cluster", clusterName)

			// Provision the cluster via usecase
			if err := clusterUC.Provision(ctx, uc.ProvisionInput{ID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to provision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "provision success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterDeprovision() *cobra.Command {
	return &cobra.Command{
		Use:   "deprovision [cluster-name]",
		Short: "Deprovision a Kubernetes cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			clusterName, err := getClusterName(cmd, args)
			if err != nil {
				return err
			}
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
			if cluster.Existing {
				logger.Info(ctx, "cluster marked as existing, skipping deprovision", "cluster", clusterName)
				return nil
			}

			logger.Info(ctx, "deprovision start", "cluster", clusterName)

			// Deprovision the cluster via usecase
			if err := clusterUC.Deprovision(ctx, uc.DeprovisionInput{ID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to deprovision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "deprovision success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterInstall() *cobra.Command {
	return &cobra.Command{
		Use:   "install [cluster-name]",
		Short: "Install cluster resources (Ingress Controller, etc.)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			clusterName, err := getClusterName(cmd, args)
			if err != nil {
				return err
			}
			clusters, err := clusterUC.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			// Find cluster by name
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
			logger.Info(ctx, "install start", "cluster", clusterName)

			// Install cluster resources via usecase
			if err := clusterUC.Install(ctx, uc.InstallInput{ID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to install cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "install success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterUninstall() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall [cluster-name]",
		Short: "Uninstall cluster resources (Ingress Controller, etc.)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			clusterName, err := getClusterName(cmd, args)
			if err != nil {
				return err
			}
			clusters, err := clusterUC.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			// Find cluster by name
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
			logger.Info(ctx, "uninstall start", "cluster", clusterName)

			// Uninstall cluster resources via usecase
			if err := clusterUC.Uninstall(ctx, uc.UninstallInput{ID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to uninstall cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "uninstall success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status [cluster-name]",
		Short: "Show cluster status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			clusterName, err := getClusterName(cmd, args)
			if err != nil {
				return err
			}
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
