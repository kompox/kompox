package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/terminal"
	"github.com/kompox/kompox/usecase/app"
	"github.com/kompox/kompox/usecase/dns"
	vuc "github.com/kompox/kompox/usecase/volume"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var flagAppName string
var flagAppID string

func newCmdApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "app",
		Short:              "Manage apps",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	// Persistent flag shared across subcommands
	cmd.PersistentFlags().StringVarP(&flagAppID, "app-id", "A", "", "App ID (FQN: ws/prv/cls/app)")
	cmd.PersistentFlags().StringVar(&flagAppName, "app-name", "", "App name (backward compatibility, use --app-id)")
	cmd.AddCommand(newCmdAppValidate(), newCmdAppDeploy(), newCmdAppDestroy(), newCmdAppStatus(), newCmdAppExec(), newCmdAppLogs())
	return cmd
}

// resolveAppID resolves the app ID from flags, config, or positional args.
// Priority: --app-id flag > --app-name flag > KOM mode default (single app) > kompoxops.yml app.name
// If the resolved value contains "/", it's treated as an FQN and returned as-is.
// Otherwise, it's treated as a name and looked up via List().
// Returns error if name matches multiple apps or no app is found.
//
// This function is shared across all commands that need app resolution.
// It uses global variables flagAppID and flagAppName set by each command's persistent flags.
func resolveAppID(ctx context.Context, appRepo domain.AppRepository, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("positional app name is not supported; use --app-id or --app-name")
	}

	// Determine the identifier from flags/config
	var idOrName string
	// 1. Use --app-id flag if provided (highest priority)
	if flagAppID != "" {
		idOrName = flagAppID
	} else if flagAppName != "" {
		// 2. Use --app-name flag if provided
		idOrName = flagAppName
	} else if komMode.enabled && komMode.defaultAppID != "" {
		// 3. In KOM mode with single app, use that app's FQN as default
		idOrName = komMode.defaultAppID
	} else if configRoot != nil && configRoot.App.Name != "" {
		// 4. In DB mode (kompoxops.yml), use app.name if available
		idOrName = configRoot.App.Name
	} else {
		return "", fmt.Errorf("app not specified; use --app-id, --app-name, or set app.name in kompoxops.yml")
	}

	// If it looks like an FQN (contains "/"), use directly as ID
	if strings.Contains(idOrName, "/") {
		return idOrName, nil
	}

	// Otherwise, find app by name
	apps, err := appRepo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list apps: %w", err)
	}

	var matches []string
	for _, a := range apps {
		if a != nil && a.Name == idOrName {
			matches = append(matches, a.ID)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("app %q not found", idOrName)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple apps found with name %q: %v (use --app-id to specify)", idOrName, matches)
	}

	return matches[0], nil
}

// normalizeYAMLDocs ensures the YAML document starts with "---" and ends with a newline.
func normalizeYAMLDocs(s string) string {
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "---\n") {
		s = "---\n" + s
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func newCmdAppValidate() *cobra.Command {
	var outComposePath string
	var outManifestPath string
	cmd := &cobra.Command{
		Use:                "validate",
		Short:              "Validate app compose definition",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			appID, err := resolveAppID(ctx, appUC.Repos.App, args)
			if err != nil {
				return err
			}

			out, err := appUC.Validate(ctx, &app.ValidateInput{AppID: appID})
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			if len(out.Errors) > 0 {
				for _, e := range out.Errors {
					logger.Error(ctx, e, "app", appID)
				}
				return fmt.Errorf("validation failed (%d errors)", len(out.Errors))
			}
			for _, w := range out.Warnings {
				logger.Warn(ctx, w, "app", appID)
			}
			if outComposePath != "" && out.Compose != "" {
				yamlDocs := normalizeYAMLDocs(out.Compose)
				if outComposePath == "-" {
					fmt.Fprint(cmd.OutOrStdout(), yamlDocs)
				} else if err := os.WriteFile(outComposePath, []byte(yamlDocs), 0o644); err != nil {
					return fmt.Errorf("failed to write compose output: %w", err)
				}
			}
			if outManifestPath != "" && len(out.K8sObjects) > 0 {
				scheme := runtime.NewScheme()
				utilruntime.Must(appsv1.AddToScheme(scheme))
				utilruntime.Must(corev1.AddToScheme(scheme))
				utilruntime.Must(netv1.AddToScheme(scheme))
				// Ensure GVKs
				for _, obj := range out.K8sObjects {
					if gvk, _, err := scheme.ObjectKinds(obj); err == nil && len(gvk) > 0 {
						obj.GetObjectKind().SetGroupVersionKind(gvk[0])
					}
				}
				manifest, err := kube.BuildCleanManifest(out.K8sObjects)
				if err != nil {
					return fmt.Errorf("failed to build manifest: %w", err)
				}
				if outManifestPath == "-" {
					fmt.Fprint(cmd.OutOrStdout(), manifest)
				} else if err := os.WriteFile(outManifestPath, []byte(manifest), 0o644); err != nil {
					return fmt.Errorf("failed to write manifest output: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outComposePath, "out-compose", "", "Write normalized compose YAML to file (omit compose YAML stdout)")
	cmd.Flags().StringVar(&outManifestPath, "out-manifest", "", "Write generated Kubernetes manifest to file (omit manifest stdout)")
	return cmd
}

// newCmdAppDeploy deploys the app's generated Kubernetes objects to its target cluster.
// Flow:
//  1. Resolve app by name
//  2. Reuse validation/conversion logic via appUC.Validate to obtain runtime.Objects
//  3. Build provider driver and fetch cluster kubeconfig (driver.ClusterKubeconfig)
//  4. Create a kube client and apply objects (create-or-update semantics where safe)
//
// Notes:
//   - PersistentVolumes and Claims are created if absent; they are left untouched if present (immutable fields)
//   - Namespace labels/annotations are merged
//   - Deployment/Service/Ingress perform create or update (simple Update with existing resourceVersion)
func newCmdAppDeploy() *cobra.Command {
	var bootstrapDisks bool
	var updateDNS bool
	cmd := &cobra.Command{
		Use:                "deploy",
		Short:              "Deploy app to cluster (apply generated Kubernetes objects)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appID, err := resolveAppID(ctx, appUC.Repos.App, args)
			if err != nil {
				return err
			}

			// Emit header log and attach context
			ctx, cleanup := withCmdRunLogger(ctx, "app.deploy", appID)
			defer func() { cleanup(err) }()

			logger := logging.FromContext(ctx)

			// Get app entity
			getOut, err := appUC.Get(ctx, &app.GetInput{AppID: appID})
			if err != nil {
				return fmt.Errorf("failed to get app: %w", err)
			}
			target := getOut.App

			if bootstrapDisks {
				volUC, verr := buildVolumeUseCase(cmd)
				if verr != nil {
					return verr
				}
				logger.Info(ctx, "bootstrap disks before deploy")
				if _, berr := volUC.DiskCreateBootstrap(ctx, &vuc.DiskCreateBootstrapInput{AppID: target.ID}); berr != nil {
					return berr
				}
			}

			if _, err := appUC.Deploy(ctx, &app.DeployInput{AppID: target.ID}); err != nil {
				return err
			}

			// Update DNS if requested
			if updateDNS {
				dnsUC, derr := buildDNSUseCase(cmd)
				if derr != nil {
					return fmt.Errorf("failed to build DNS use case: %w", derr)
				}
				logger.Info(ctx, "updating DNS records")
				_, derr = dnsUC.Deploy(ctx, &dns.DeployInput{
					AppID:         target.ID,
					ComponentName: "app",
				})
				if derr != nil {
					return fmt.Errorf("failed to update DNS: %w", derr)
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&bootstrapDisks, "bootstrap-disks", false, "Create one assigned disk per volume if none exist (fails on partial state)")
	cmd.Flags().BoolVar(&updateDNS, "update-dns", false, "Update DNS records after deployment")
	return cmd
}

// newCmdAppDestroy removes deployed Kubernetes resources selected by labels and optionally deletes the Namespace.
// Behavior:
//   - Deletes only resources labeled with both:
//     app.kubernetes.io/instance = <appName>-<inHASH>
//     app.kubernetes.io/managed-by = kompox
//   - By default deletes all namespaced resources (matching labels) and PV/PVC. Namespace itself is NOT deleted.
//   - When --delete-namespace is provided, also delete the Namespace resource.
func newCmdAppDestroy() *cobra.Command {
	var deleteNamespace bool
	var updateDNS bool
	cmd := &cobra.Command{
		Use:                "destroy",
		Short:              "Destroy app resources from cluster (label-selected delete)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appID, err := resolveAppID(ctx, appUC.Repos.App, args)
			if err != nil {
				return err
			}

			// Emit header log and attach context
			ctx, cleanup := withCmdRunLogger(ctx, "app.destroy", appID)
			defer func() { cleanup(err) }()

			logger := logging.FromContext(ctx)

			// Get app entity
			getOut, err := appUC.Get(ctx, &app.GetInput{AppID: appID})
			if err != nil {
				return fmt.Errorf("failed to get app: %w", err)
			}
			target := getOut.App

			// Destroy DNS records if requested (before destroying the app)
			if updateDNS {
				dnsUC, derr := buildDNSUseCase(cmd)
				if derr != nil {
					return fmt.Errorf("failed to build DNS use case: %w", derr)
				}
				logger.Info(ctx, "destroying DNS records")
				_, derr = dnsUC.Destroy(ctx, &dns.DestroyInput{
					AppID:         target.ID,
					ComponentName: "app",
				})
				if derr != nil {
					return fmt.Errorf("failed to destroy DNS: %w", derr)
				}
			}

			if _, err := appUC.Destroy(ctx, &app.DestroyInput{AppID: target.ID, DeleteNamespace: deleteNamespace}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&deleteNamespace, "delete-namespace", false, "Also delete the Namespace resource")
	cmd.Flags().BoolVar(&updateDNS, "update-dns", false, "Destroy DNS records before destroying the app")
	return cmd
}

// newCmdAppStatus shows app status as JSON (ingress hostnames, etc.).
func newCmdAppStatus() *cobra.Command {
	return &cobra.Command{
		Use:                "status",
		Short:              "Show app status (ingress hosts, etc.)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			appID, err := resolveAppID(ctx, appUC.Repos.App, args)
			if err != nil {
				return err
			}

			st, err := appUC.Status(ctx, &app.StatusInput{AppID: appID})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(st)
		},
	}
}

// newCmdAppExec executes a command in a selected pod of the app namespace.
func newCmdAppExec() *cobra.Command {
	var stdin bool
	var tty bool
	var escape string
	var container string
	cmd := &cobra.Command{
		Use:                "exec -- <command> [args...]",
		Short:              "Exec into an app pod",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// localize klog suppression only for exec
			terminal.QuietKlog()
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			appID, err := resolveAppID(ctx, appUC.Repos.App, nil)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return fmt.Errorf("command is required after --")
			}
			_, err = appUC.Exec(ctx, &app.ExecInput{AppID: appID, Command: args, Stdin: stdin, TTY: tty, Escape: escape, Container: container})
			return err
		},
	}
	cmd.Flags().BoolVarP(&stdin, "stdin", "i", false, "Pass stdin to the command")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a TTY")
	cmd.Flags().StringVarP(&escape, "escape", "e", "~.", "Escape sequence to detach (e.g. '~.'); set 'none' to disable")
	cmd.Flags().StringVarP(&container, "container", "c", "", "Container name (optional)")
	return cmd
}

// newCmdAppLogs streams or prints logs from one pod of the app namespace.
// Selection strategy matches exec: prefer a Ready non tool-runner pod.
func newCmdAppLogs() *cobra.Command {
	var follow bool
	var tail int64
	var container string
	cmd := &cobra.Command{
		Use:                "logs",
		Short:              "Show logs from an app pod",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			terminal.QuietKlog()
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}

			appID, err := resolveAppID(cmd.Context(), appUC.Repos.App, nil)
			if err != nil {
				return err
			}

			in := &app.LogsInput{AppID: appID, Container: container, Follow: follow}
			if tail > 0 {
				in.TailLines = &tail
			}
			// Do not impose a timeout when following
			ctx := cmd.Context()
			if !follow {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
			}
			_, err = appUC.Logs(ctx, in)
			if err != nil {
				// treat context cancellation while following as normal termination
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
