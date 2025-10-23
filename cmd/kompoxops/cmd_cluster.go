package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
	kcfg "github.com/kompox/kompox/internal/kubeconfig"
	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/naming"
	uc "github.com/kompox/kompox/usecase/cluster"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var flagClusterName string
var flagClusterID string

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
	// Persistent flag so all subcommands can use --cluster-name / -C / --cluster-id
	cmd.PersistentFlags().StringVarP(&flagClusterID, "cluster-id", "C", "", "Cluster ID (FQN: ws/prv/cls)")
	cmd.PersistentFlags().StringVar(&flagClusterName, "cluster-name", "", "Cluster name (backward compatibility, use --cluster-id)")
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

// resolveClusterID resolves a cluster identifier to a Cluster ID (FQN).
// Priority: --cluster-id flag > --cluster-name flag > KOM mode default > kompoxops.yml default > positional arg
// If the identifier contains "/", it's treated as an FQN and returned as-is.
// Otherwise, it's treated as a name and looked up via List().
// Returns error if name matches multiple clusters or no cluster is found.
func resolveClusterID(ctx context.Context, clusterRepo domain.ClusterRepository, args []string) (string, error) {
	// Determine the identifier from flags/config/args
	var idOrName string
	if flagClusterID != "" {
		idOrName = flagClusterID
	} else if flagClusterName != "" {
		idOrName = flagClusterName
	} else if komMode.enabled && komMode.defaultClusterID != "" {
		idOrName = komMode.defaultClusterID
	} else if configRoot != nil && configRoot.Cluster.Name != "" {
		idOrName = configRoot.Cluster.Name
	} else if len(args) > 0 {
		return "", fmt.Errorf("positional cluster name is not supported; use --cluster-id or --cluster-name")
	} else {
		return "", fmt.Errorf("cluster not specified; use --cluster-id, --cluster-name, or set cluster.name in kompoxops.yml")
	}

	// If it looks like an FQN (contains "/"), use directly as ID
	if strings.Contains(idOrName, "/") {
		return idOrName, nil
	}

	// Otherwise, find cluster by name
	clusters, err := clusterRepo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list clusters: %w", err)
	}

	var matches []string
	for _, c := range clusters {
		if c != nil && c.Name == idOrName {
			matches = append(matches, c.ID)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("cluster %q not found", idOrName)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple clusters found with name %q: %v (use --cluster-id to specify)", idOrName, matches)
	}

	return matches[0], nil
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

			rctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			clusterID, err := resolveClusterID(rctx, clusterUC.Repos.Cluster, nil)
			if err != nil {
				return err
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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			cluster := getOut.Cluster
			if cluster.Existing && !force {
				logger.Info(ctx, "cluster marked as existing, skipping provision", "cluster", cluster.Name)
				return nil
			}

			logger.Info(ctx, "provision start", "cluster", cluster.Name)

			// Provision the cluster via usecase
			if _, err := clusterUC.Provision(ctx, &uc.ProvisionInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to provision cluster %s: %w", cluster.Name, err)
			}

			logger.Info(ctx, "provision success", "cluster", cluster.Name)
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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			cluster := getOut.Cluster
			if cluster.Existing && !force {
				logger.Info(ctx, "cluster marked as existing, skipping deprovision", "cluster", cluster.Name)
				return nil
			}

			// Early guard: check protection policy
			if err := cluster.CheckProvisioningProtection("deprovision"); err != nil {
				return err
			}

			logger.Info(ctx, "deprovision start", "cluster", cluster.Name)

			// Deprovision the cluster via usecase
			if _, err := clusterUC.Deprovision(ctx, &uc.DeprovisionInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to deprovision cluster %s: %w", cluster.Name, err)
			}

			logger.Info(ctx, "deprovision success", "cluster", cluster.Name)
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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			cluster := getOut.Cluster

			logger.Info(ctx, "install start", "cluster", cluster.Name)

			// Install cluster resources via usecase
			if _, err := clusterUC.Install(ctx, &uc.InstallInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to install cluster resources for %s: %w", cluster.Name, err)
			}

			logger.Info(ctx, "install success", "cluster", cluster.Name)
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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			cluster := getOut.Cluster

			// Early guard: check protection policy
			if err := cluster.CheckInstallationProtection("uninstall", false); err != nil {
				return err
			}

			logger.Info(ctx, "uninstall start", "cluster", cluster.Name)

			// Uninstall cluster resources via usecase
			if _, err := clusterUC.Uninstall(ctx, &uc.UninstallInput{ClusterID: cluster.ID, Force: force}); err != nil {
				return fmt.Errorf("failed to uninstall cluster resources for %s: %w", cluster.Name, err)
			}

			logger.Info(ctx, "uninstall success", "cluster", cluster.Name)
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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			cluster := getOut.Cluster

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

			clusterID, err := resolveClusterID(ctx, clusterUC.Repos.Cluster, args)
			if err != nil {
				return err
			}

			getOut, err := clusterUC.Get(ctx, &uc.GetInput{ClusterID: clusterID})
			if err != nil {
				return fmt.Errorf("failed to get cluster: %w", err)
			}
			clusterObj := getOut.Cluster

			// Build provider driver and fetch kubeconfig bytes
			providerObj, err := clusterUC.Repos.Provider.Get(ctx, clusterObj.ProviderID)
			if err != nil || providerObj == nil {
				return fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
			}
			var workspaceObj *model.Workspace
			if providerObj.WorkspaceID != "" {
				workspaceObj, _ = clusterUC.Repos.Workspace.Get(ctx, providerObj.WorkspaceID)
			}
			factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
			if !ok {
				return fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
			}
			drv, err := factory(workspaceObj, providerObj)
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
				contextName = fmt.Sprintf("kompoxops-%s", clusterObj.Name)
			}
			// Derive default namespace if not explicitly provided via --namespace.
			// We use naming.NewHashes(workspace, provider, cluster, app).Namespace when
			// configRoot is available and app name configured; otherwise keep empty to
			// preserve upstream kubeconfig default namespace behavior.
			if namespaceName == "" && configRoot != nil {
				workspaceName := configRoot.Workspace.Name
				providerName := providerObj.Name
				appName := configRoot.App.Name
				if workspaceName != "" && providerName != "" && appName != "" { // require all to avoid accidental collision
					hashes := naming.NewHashes(workspaceName, providerName, clusterObj.Name, appName)
					namespaceName = hashes.Namespace
				}
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
	cmd.Flags().StringVar(&namespaceName, "namespace", "", "Default namespace for the context (auto-derived when omitted)")
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set current-context to the new context after merge")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite same-named entries when merging")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format for stdout (yaml|json)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show intended merge changes without writing")
	return cmd
}
