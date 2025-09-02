package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/terminal"
	"github.com/kompox/kompox/usecase/app"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var flagAppName string

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
	cmd.PersistentFlags().StringVarP(&flagAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.AddCommand(newCmdAppValidate(), newCmdAppDeploy(), newCmdAppDestroy(), newCmdAppStatus(), newCmdAppExec())
	return cmd
}

// getAppName resolves the app name from flag or config file. Positional args are no longer supported.
func getAppName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("positional app name is not supported; use --app-name")
	}
	if flagAppName != "" {
		return flagAppName, nil
	}
	if configRoot != nil && len(configRoot.App.Name) > 0 {
		return configRoot.App.Name, nil
	}
	return "", fmt.Errorf("app name not specified; use --app-name or set app.name in kompoxops.yml")
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

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			// Find app by name using new List Input/Output pattern
			listOut, err := appUC.List(ctx, &app.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var target *string
			for _, a := range listOut.Apps {
				if a.Name == appName {
					id := a.ID
					target = &id
					break
				}
			}
			if target == nil {
				return fmt.Errorf("app %s not found", appName)
			}
			out, err := appUC.Validate(ctx, &app.ValidateInput{AppID: *target})
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			if len(out.Errors) > 0 {
				for _, e := range out.Errors {
					logger.Error(ctx, e, "app", appName)
				}
				return fmt.Errorf("validation failed (%d errors)", len(out.Errors))
			}
			for _, w := range out.Warnings {
				logger.Warn(ctx, w, "app", appName)
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
	cmd := &cobra.Command{
		Use:                "deploy",
		Short:              "Deploy app to cluster (apply generated Kubernetes objects)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			// Resolve app by name
			listOut, err := appUC.List(ctx, &app.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var target *model.App
			for _, a := range listOut.Apps {
				if a.Name == appName {
					target = a
					break
				}
			}
			if target == nil {
				return fmt.Errorf("app %s not found", appName)
			}

			if _, err := appUC.Deploy(ctx, &app.DeployInput{AppID: target.ID}); err != nil {
				return err
			}
			return nil
		},
	}
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
	cmd := &cobra.Command{
		Use:                "destroy",
		Short:              "Destroy app resources from cluster (label-selected delete)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appUC, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			// Find app by name
			listOut, err := appUC.List(ctx, &app.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var target *model.App
			for _, a := range listOut.Apps {
				if a.Name == appName {
					target = a
					break
				}
			}
			if target == nil {
				return fmt.Errorf("app %s not found", appName)
			}

			if _, err := appUC.Destroy(ctx, &app.DestroyInput{AppID: target.ID, DeleteNamespace: deleteNamespace}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&deleteNamespace, "delete-namespace", false, "Also delete the Namespace resource")
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

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}

			// Find app by name
			listOut, err := appUC.List(ctx, &app.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var targetID string
			for _, a := range listOut.Apps {
				if a.Name == appName {
					targetID = a.ID
					break
				}
			}
			if targetID == "" {
				return fmt.Errorf("app %s not found", appName)
			}

			st, err := appUC.Status(ctx, &app.StatusInput{AppID: targetID})
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
			appName, err := getAppName(cmd, nil)
			if err != nil {
				return err
			}
			// Resolve app by name using a short-lived context
			var appID string
			{
				rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				listOut, err := appUC.List(rctx, &app.ListInput{})
				if err != nil {
					return fmt.Errorf("failed to list apps: %w", err)
				}
				for _, a := range listOut.Apps {
					if a.Name == appName {
						appID = a.ID
						break
					}
				}
			}
			if appID == "" {
				return fmt.Errorf("app %s not found", appName)
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
