package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	uc "github.com/kompox/kompox/usecase/cluster"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	yaml "sigs.k8s.io/yaml"
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
//  5. Else write to --out path or stdout; --temp writes to secure temp file and prints its path
func newCmdClusterKubeconfig() *cobra.Command {
	var (
		outPath      string
		mergeInto    bool
		kubeconfigIn string
		contextName  string
		namespace    string
		setCurrent   bool
		force        bool
		useTemp      bool
		printExport  bool
		format       string
		dryRun       bool
		credType     string
		timeout      time.Duration
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
			if printExport && !(useTemp || (outPath != "" && outPath != "-")) {
				return fmt.Errorf("--print-export requires --temp or --out <path>")
			}
			if credType != "admin" {
				return fmt.Errorf("credential type %q not supported; only 'admin' is available currently", credType)
			}
			// Require an explicit destination: --merge, --temp, or --out (including -o - for stdout)
			if !mergeInto && !useTemp && outPath == "" {
				return fmt.Errorf("no output target specified; use --merge, --temp, or --out <path> (use -o - for stdout)")
			}

			clusterUC, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
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
			newCfg, baseCtxName, err := loadAndNormalizeKubeconfig(kubeBytes, contextName, namespace, clusterName)
			if err != nil {
				return err
			}

			// Merge or output
			if mergeInto {
				if kubeconfigIn == "" {
					// default to ~/.kube/config
					home, _ := os.UserHomeDir()
					kubeconfigIn = filepath.Join(home, ".kube", "config")
				}
				merged, finalCtx, changed, err := mergeIntoExistingKubeconfig(newCfg, kubeconfigIn, baseCtxName, force, setCurrent, dryRun)
				if err != nil {
					return err
				}
				if dryRun {
					// Print a short summary of intended changes
					fmt.Fprintf(cmd.OutOrStdout(), "dry-run: will %s %d entry(ies); context=%s current=%v\n", changed.action, changed.count, finalCtx, changed.current)
					return nil
				}
				// Persist merged config
				if err := writeKubeconfigFile(kubeconfigIn, merged); err != nil {
					return err
				}
				// Optionally print export line if requested and file path is known
				if printExport {
					fmt.Fprintf(cmd.OutOrStdout(), "export KUBECONFIG=%s\n", kubeconfigIn)
				}
				return nil
			}

			// Not merging: write to stdout, file, or temp
			var path string
			if useTemp {
				p, err := writeTempKubeconfigFile(newCfg)
				if err != nil {
					return err
				}
				path = p
			} else if outPath != "" && outPath != "-" {
				if err := writeKubeconfigFile(outPath, newCfg); err != nil {
					return err
				}
				path = outPath
			}

			if printExport {
				if path == "" {
					return fmt.Errorf("--print-export requires --temp or --out <path>")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "export KUBECONFIG=%s\n", path)
				return nil
			}

			// stdout only when explicitly requested via -o -
			if outPath == "-" {
				if err := printKubeconfig(cmd.OutOrStdout(), newCfg, format); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&outPath, "out", "o", "", "Write kubeconfig to file ('-' for stdout; requires explicit -o -)")
	cmd.Flags().BoolVar(&mergeInto, "merge", false, "Merge into existing kubeconfig (default: ~/.kube/config)")
	cmd.Flags().StringVar(&kubeconfigIn, "kubeconfig", "", "Existing kubeconfig path for --merge (default: ~/.kube/config)")
	cmd.Flags().StringVar(&contextName, "context", "", "Context name to set (default: kompoxops-<cluster>)")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Default namespace for the context")
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set current-context to the new context after merge")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite same-named entries when merging")
	cmd.Flags().BoolVar(&useTemp, "temp", false, "Write to a secure temporary file and print its path")
	cmd.Flags().BoolVar(&printExport, "print-export", false, "Print 'export KUBECONFIG=...' for shell usage")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format for stdout (yaml|json)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show intended merge changes without writing")
	cmd.Flags().StringVar(&credType, "cred", "admin", "Credential type to request (admin|user)")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Timeout for retrieving kubeconfig")
	return cmd
}

// ===== Kubeconfig helpers (internal) =====

// Note: Go comments must be English per repo instructions.

// loadAndNormalizeKubeconfig loads kubeconfig bytes and returns a minimal config
// containing a single context, cluster, and authinfo. Optionally renames context
// and sets its default namespace.
func loadAndNormalizeKubeconfig(data []byte, desiredCtx, ns, clusterName string) (*clientcmdapi.Config, string, error) {
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return nil, "", fmt.Errorf("parse kubeconfig: %w", err)
	}
	// Determine base context
	ctxName := cfg.CurrentContext
	if ctxName == "" && len(cfg.Contexts) == 1 {
		for k := range cfg.Contexts { // pick the only one
			ctxName = k
		}
	}
	if ctxName == "" {
		return nil, "", fmt.Errorf("kubeconfig has no current context")
	}
	ctx := cfg.Contexts[ctxName]
	if ctx == nil {
		return nil, "", fmt.Errorf("context %q not found in kubeconfig", ctxName)
	}
	// Build a minimal config
	min := clientcmdapi.NewConfig()
	// Decide final names
	finalCtx := desiredCtx
	if finalCtx == "" {
		finalCtx = fmt.Sprintf("kompoxops-%s", clusterName)
	}
	// Use same names for cluster and authinfo basing on context for simplicity
	finalCluster := finalCtx
	finalUser := finalCtx

	// Copy referenced cluster
	if srcCluster, ok := cfg.Clusters[ctx.Cluster]; ok && srcCluster != nil {
		min.Clusters[finalCluster] = &clientcmdapi.Cluster{
			Server:                   srcCluster.Server,
			InsecureSkipTLSVerify:    srcCluster.InsecureSkipTLSVerify,
			CertificateAuthority:     srcCluster.CertificateAuthority,
			CertificateAuthorityData: append([]byte(nil), srcCluster.CertificateAuthorityData...),
			ProxyURL:                 srcCluster.ProxyURL,
			TLSServerName:            srcCluster.TLSServerName,
		}
	} else {
		return nil, "", fmt.Errorf("referenced cluster %q not found", ctx.Cluster)
	}
	// Copy referenced authinfo
	if srcUser, ok := cfg.AuthInfos[ctx.AuthInfo]; ok && srcUser != nil {
		// Deep copy relevant fields
		dst := &clientcmdapi.AuthInfo{
			ClientCertificate:     srcUser.ClientCertificate,
			ClientCertificateData: append([]byte(nil), srcUser.ClientCertificateData...),
			ClientKey:             srcUser.ClientKey,
			ClientKeyData:         append([]byte(nil), srcUser.ClientKeyData...),
			Token:                 srcUser.Token,
			TokenFile:             srcUser.TokenFile,
			Impersonate:           srcUser.Impersonate,
			ImpersonateUID:        srcUser.ImpersonateUID,
			ImpersonateGroups:     append([]string(nil), srcUser.ImpersonateGroups...),
			ImpersonateUserExtra:  map[string][]string{},
			Username:              srcUser.Username,
			Password:              srcUser.Password,
			AuthProvider:          nil,
			Exec:                  nil,
		}
		for k, v := range srcUser.ImpersonateUserExtra {
			dst.ImpersonateUserExtra[k] = append([]string(nil), v...)
		}
		if srcUser.AuthProvider != nil {
			// Shallow copy; fields are simple types
			ap := *srcUser.AuthProvider
			dst.AuthProvider = &ap
		}
		if srcUser.Exec != nil {
			ex := *srcUser.Exec
			dst.Exec = &ex
		}
		min.AuthInfos[finalUser] = dst
	} else {
		return nil, "", fmt.Errorf("referenced user %q not found", ctx.AuthInfo)
	}

	// Build context
	min.Contexts[finalCtx] = &clientcmdapi.Context{
		Cluster:   finalCluster,
		AuthInfo:  finalUser,
		Namespace: ns,
	}
	min.CurrentContext = finalCtx
	return min, finalCtx, nil
}

type mergeChange struct {
	action  string
	count   int
	current bool
}

// mergeIntoExistingKubeconfig merges newCfg into an existing kubeconfig file path.
// It resolves name conflicts; if force=false, it will suffix -1, -2... to obtain unique names.
// Returns merged config, final context name, a change summary, and error.
func mergeIntoExistingKubeconfig(newCfg *clientcmdapi.Config, path, baseCtx string, force, setCurrent, dryRun bool) (*clientcmdapi.Config, string, mergeChange, error) {
	// Load existing; if not exists, start from empty
	var existing *clientcmdapi.Config
	if b, err := os.ReadFile(path); err == nil {
		cfg, err := clientcmd.Load(b)
		if err != nil {
			return nil, "", mergeChange{}, fmt.Errorf("parse existing kubeconfig: %w", err)
		}
		existing = cfg
	} else {
		existing = clientcmdapi.NewConfig()
		// ensure dir exists
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, "", mergeChange{}, fmt.Errorf("prepare kubeconfig directory: %w", err)
		}
	}

	// Determine unique names
	ctxName := newCfg.CurrentContext
	clusterName := newCfg.Contexts[ctxName].Cluster
	userName := newCfg.Contexts[ctxName].AuthInfo

	if !force {
		ctxName = uniqueName(ctxName, existing.Contexts)
		clusterName = uniqueName(clusterName, existing.Clusters)
		userName = uniqueName(userName, existing.AuthInfos)
	}

	// If force and names exist, drop them first
	if force {
		delete(existing.Contexts, ctxName)
		delete(existing.Clusters, clusterName)
		delete(existing.AuthInfos, userName)
	}

	// Insert copies under decided names
	// Clusters
	if c := newCfg.Clusters[newCfg.Contexts[newCfg.CurrentContext].Cluster]; c != nil {
		existing.Clusters[clusterName] = &clientcmdapi.Cluster{
			Server:                   c.Server,
			InsecureSkipTLSVerify:    c.InsecureSkipTLSVerify,
			CertificateAuthority:     c.CertificateAuthority,
			CertificateAuthorityData: append([]byte(nil), c.CertificateAuthorityData...),
			ProxyURL:                 c.ProxyURL,
			TLSServerName:            c.TLSServerName,
		}
	}
	// AuthInfos
	if u := newCfg.AuthInfos[newCfg.Contexts[newCfg.CurrentContext].AuthInfo]; u != nil {
		dst := *u // shallow copy sufficient; contained slices already copied in load step
		existing.AuthInfos[userName] = &dst
	}
	// Context
	existing.Contexts[ctxName] = &clientcmdapi.Context{
		Cluster:   clusterName,
		AuthInfo:  userName,
		Namespace: newCfg.Contexts[newCfg.CurrentContext].Namespace,
	}
	change := mergeChange{action: "add/update", count: 3, current: false}
	// Auto-select the new context when the existing config has no current context yet
	if setCurrent || existing.CurrentContext == "" {
		existing.CurrentContext = ctxName
		change.current = true
	}
	return existing, ctxName, change, nil
}

// writeKubeconfigFile writes config to path with 0600 permission.
func writeKubeconfigFile(path string, cfg *clientcmdapi.Config) error {
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return fmt.Errorf("serialize kubeconfig: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig: %w", err)
	}
	return nil
}

// writeTempKubeconfigFile creates a secure temp file (0600) with kubeconfig and returns its path.
func writeTempKubeconfigFile(cfg *clientcmdapi.Config) (string, error) {
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return "", fmt.Errorf("serialize kubeconfig: %w", err)
	}
	f, err := os.CreateTemp("", "kompoxops-kubeconfig-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return f.Name(), nil
}

// printKubeconfig prints cfg to writer in yaml or json.
func printKubeconfig(w io.Writer, cfg *clientcmdapi.Config, format string) error {
	data, err := clientcmd.Write(*cfg)
	if err != nil {
		return fmt.Errorf("serialize kubeconfig: %w", err)
	}
	if format == "json" {
		// convert kubeconfig YAML to JSON
		j, err := yaml.YAMLToJSON(data)
		if err != nil {
			return fmt.Errorf("convert to json: %w", err)
		}
		_, err = w.Write(j)
		return err
	}
	_, err = w.Write(data)
	return err
}

// uniqueName returns name if not present in m; otherwise appends -1, -2... until unique.
func uniqueName[T any](name string, m map[string]T) string {
	if _, ok := m[name]; !ok {
		return name
	}
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d", name, i)
		if _, ok := m[cand]; !ok {
			return cand
		}
	}
}
