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

var flagClusterName string

func newCmdCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "cluster",
		Short:              "Manage clusters",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}
	// Persistent flag so all subcommands can use --cluster-name / -C
	cmd.PersistentFlags().StringVarP(&flagClusterName, "cluster-name", "C", "", "Cluster name (default: cluster.name in kompoxops.yml)")
	cmd.AddCommand(
		newCmdClusterProvision(),
		newCmdClusterDeprovision(),
		newCmdClusterInstall(),
		newCmdClusterUninstall(),
		newCmdClusterStatus(),
	)
	return cmd
}

// getClusterName resolves the cluster name from flag or config file. Positional args are no longer supported.
func getClusterName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("positional cluster name is not supported; use --cluster-name")
	}
	if flagClusterName != "" { // explicit flag
		return flagClusterName, nil
	}
	if configRoot != nil && len(configRoot.Cluster.Name) > 0 { // default from config
		return configRoot.Cluster.Name, nil
	}
	return "", fmt.Errorf("cluster name not specified; use --cluster-name or set cluster.name in kompoxops.yml")
}

func newCmdClusterProvision() *cobra.Command {
	return &cobra.Command{
		Use:                "provision",
		Short:              "Provision a Kubernetes cluster",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
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
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			var cluster *model.Cluster
			for _, c := range listOut.Clusters {
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
			if _, err := clusterUC.Provision(ctx, &uc.ProvisionInput{ClusterID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to provision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "provision success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterDeprovision() *cobra.Command {
	return &cobra.Command{
		Use:                "deprovision",
		Short:              "Deprovision a Kubernetes cluster",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
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
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			var cluster *model.Cluster
			for _, c := range listOut.Clusters {
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
			if _, err := clusterUC.Deprovision(ctx, &uc.DeprovisionInput{ClusterID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to deprovision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "deprovision success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterInstall() *cobra.Command {
	return &cobra.Command{
		Use:                "install",
		Short:              "Install cluster resources (Ingress Controller, etc.)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
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
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			// Find cluster by name
			var cluster *model.Cluster
			for _, c := range listOut.Clusters {
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
			if _, err := clusterUC.Install(ctx, &uc.InstallInput{ClusterID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to install cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "install success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterUninstall() *cobra.Command {
	return &cobra.Command{
		Use:                "uninstall",
		Short:              "Uninstall cluster resources (Ingress Controller, etc.)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
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
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			// Find cluster by name
			var cluster *model.Cluster
			for _, c := range listOut.Clusters {
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
			if _, err := clusterUC.Uninstall(ctx, &uc.UninstallInput{ClusterID: cluster.ID}); err != nil {
				return fmt.Errorf("failed to uninstall cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "uninstall success", "cluster", clusterName)
			return nil
		},
	}
}

func newCmdClusterStatus() *cobra.Command {
	return &cobra.Command{
		Use:                "status",
		Short:              "Show cluster status",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
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
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}

			var cluster *model.Cluster
			for _, c := range listOut.Clusters {
				if c.Name == clusterName {
					cluster = c
					break
				}
			}
			if cluster == nil {
				return fmt.Errorf("cluster %s not found", clusterName)
			}

			// Get status
			status, err := clusterUC.Status(ctx, &uc.StatusInput{ClusterID: cluster.ID})
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
