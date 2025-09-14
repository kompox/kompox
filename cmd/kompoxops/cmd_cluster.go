package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/domain/model"
	kcfg "github.com/kompox/kompox/internal/kubeconfig"
	"github.com/kompox/kompox/internal/logging"
	uc "github.com/kompox/kompox/usecase/cluster"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
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
		newCmdClusterKubeconfig(),
		newCmdClusterLogs(),
	)
	return cmd
}

// newCmdClusterLogs streams or prints logs from a traefik ingress pod in the cluster.
func newCmdClusterLogs() *cobra.Command {
	var follow bool
	var tail int64
	var container string
	cmd := &cobra.Command{
		Use:                "logs",
		Short:              "Show logs from a traefik ingress pod",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}
			clusterName, err := getClusterName(cmd, nil)
			if err != nil {
				return err
			}
			// Find cluster by name
			var clusterID string
			{
				rctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
				defer cancel()
				listOut, err := clusterUC.List(rctx, &uc.ListInput{})
				if err != nil {
					return fmt.Errorf("failed to list clusters: %w", err)
				}
				for _, c := range listOut.Clusters {
					if c.Name == clusterName {
						clusterID = c.ID
						break
					}
				}
			}
			if clusterID == "" {
				return fmt.Errorf("cluster %s not found", clusterName)
			}
			in := &uc.LogsInput{ClusterID: clusterID, Container: container, Follow: follow}
			if tail > 0 {
				in.TailLines = &tail
			}
			ctx := cmd.Context()
			if !follow {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
			}
			_, err = clusterUC.Logs(ctx, in)
			if err != nil {
				if follow && ctx.Err() == context.Canceled {
					return nil
				}
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream logs (follow)")
	cmd.Flags().Int64Var(&tail, "tail", 200, "Number of lines from the end of the logs to show (0 to show all)")
	cmd.Flags().StringVarP(&container, "container", "c", "", "Container name (optional)")
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
	var force bool
	cmd := &cobra.Command{
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
			if _, err := clusterUC.Provision(ctx, &uc.ProvisionInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to provision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "provision success", "cluster", clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force cluster provisioning even if a successful deployment exists")
	return cmd
}

func newCmdClusterDeprovision() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
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
			if _, err := clusterUC.Deprovision(ctx, &uc.DeprovisionInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to deprovision cluster %s: %w", clusterName, err)
			}

			logger.Info(ctx, "deprovision success", "cluster", clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force deprovision behavior if driver supports it")
	return cmd
}

func newCmdClusterInstall() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
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
			if _, err := clusterUC.Install(ctx, &uc.InstallInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to install cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "install success", "cluster", clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force install behavior if driver supports it")
	return cmd
}

func newCmdClusterUninstall() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
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
			if _, err := clusterUC.Uninstall(ctx, &uc.UninstallInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to uninstall cluster resources for %s: %w", clusterName, err)
			}

			logger.Info(ctx, "uninstall success", "cluster", clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force uninstall behavior if driver supports it")
	return cmd
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

// newCmdClusterKubeconfig fetches kubeconfig bytes from the provider driver and outputs or merges them.
// Flow:
//  1. Resolve target cluster by name
//  2. Build provider driver and call ClusterKubeconfig(ctx, cluster)
//  3. Optionally rewrite context name/namespace and prune entries to a single context
//  4. If --merge, merge into existing kubeconfig file with conflict resolution
//  5. Else write to --out path or stdout
func newCmdClusterKubeconfig() *cobra.Command {
	var (
		outPath        string
		merge          bool
		kubeconfigPath string
		contextName    string
		namespaceName  string
		setCurrent     bool
		force          bool
		format         string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:                "kubeconfig",
		Short:              "Get or merge kubeconfig for the target cluster",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate flags
			if format != "yaml" && format != "json" {
				return fmt.Errorf("invalid --format: %s (yaml|json)", format)
			}
			// Require an explicit destination: --merge or --out (including -o - for stdout)
			if !merge && outPath == "" {
				return fmt.Errorf("no output target specified; use --merge or --out <path> (use -o - for stdout)")
			}

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
			// Find cluster by name
			listOut, err := clusterUC.List(ctx, &uc.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list clusters: %w", err)
			}
			var clusterObj *model.Cluster
			for _, c := range listOut.Clusters {
				if c.Name == clusterName {
					clusterObj = c
					break
				}
			}
			if clusterObj == nil {
				return fmt.Errorf("cluster %s not found", clusterName)
			}

			// Build provider driver and fetch kubeconfig bytes
			providerObj, err := clusterUC.Repos.Provider.Get(ctx, clusterObj.ProviderID)
			if err != nil || providerObj == nil {
				return fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
			}
			var serviceObj *model.Service
			if providerObj.ServiceID != "" {
				serviceObj, _ = clusterUC.Repos.Service.Get(ctx, providerObj.ServiceID)
			}
			factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
			if !ok {
				return fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
			}
			drv, err := factory(serviceObj, providerObj)
			if err != nil {
				return fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
			}
			kubeBytes, err := drv.ClusterKubeconfig(ctx, clusterObj)
			if err != nil {
				return fmt.Errorf("failed to get cluster kubeconfig: %w", err)
			}

			// Load new kubeconfig and normalize
			// Decide default context name when not provided
			if contextName == "" {
				contextName = fmt.Sprintf("kompoxops-%s", clusterName)
			}
			newCfg, err := kcfg.LoadAndNormalize(kubeBytes, contextName, namespaceName)
			if err != nil {
				return err
			}

			// Merge or output
			if merge {
				if kubeconfigPath == "" {
					// Determine precedence using client-go loading rules (KUBECONFIG env or default path)
					rules := clientcmd.NewDefaultClientConfigLoadingRules()
					prec := rules.GetLoadingPrecedence()
					if len(prec) > 0 {
						kubeconfigPath = prec[0]
					} else {
						kubeconfigPath = clientcmd.RecommendedHomeFile
					}
				}
				merged, finalCtx, changed, err := kcfg.MergeIntoExisting(newCfg, kubeconfigPath, force, setCurrent)
				if err != nil {
					return err
				}
				if dryRun {
					// Print a short summary of intended changes
					fmt.Fprintf(cmd.OutOrStdout(), "dry-run: will %s %d entry(ies); context=%s current=%v\n", changed.Action, changed.Count, finalCtx, changed.Current)
					return nil
				}
				// Persist merged config
				if err := clientcmd.WriteToFile(*merged, kubeconfigPath); err != nil {
					return err
				}
				return nil
			}

			// Not merging: write to stdout or file
			if outPath != "" && outPath != "-" {
				if err := clientcmd.WriteToFile(*newCfg, outPath); err != nil {
					return err
				}
				return nil
			}

			// stdout only when explicitly requested via -o -
			if outPath == "-" {
				if err := kcfg.Print(cmd.OutOrStdout(), newCfg, format); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outPath, "out", "o", "", "Write kubeconfig to file ('-' for stdout; requires explicit -o -)")
	cmd.Flags().BoolVar(&merge, "merge", false, "Merge into existing kubeconfig")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", fmt.Sprintf("Existing kubeconfig path for --merge (default: $%s or %s)", clientcmd.RecommendedConfigPathEnvVar, clientcmd.RecommendedHomeFile))
	cmd.Flags().StringVar(&contextName, "context", "", "Context name to set (default: kompoxops-<cluster>)")
	cmd.Flags().StringVar(&namespaceName, "namespace", "", "Default namespace for the context")
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set current-context to the new context after merge")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite same-named entries when merging")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format for stdout (yaml|json)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show intended merge changes without writing")
	return cmd
}
