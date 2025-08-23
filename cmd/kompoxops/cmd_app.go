package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"
	"github.com/yaegashi/kompoxops/usecase/app"
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
	cmd.AddCommand(newCmdAppValidate(), newCmdAppDeploy())
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
			logger := logging.FromContext(ctx)

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			// Find app by name
			listOut, err := appUC.List(ctx, &app.ListInput{})
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var target *model.App // need ID only
			for _, a := range listOut.Apps {
				if a.Name == appName {
					target = a
					break
				}
			}
			if target == nil {
				return fmt.Errorf("app %s not found", appName)
			}

			// Perform validation + conversion to get objects (and volume instance resolutions)
			vout, err := appUC.Validate(ctx, &app.ValidateInput{AppID: target.ID})
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			if len(vout.Errors) > 0 {
				for _, e := range vout.Errors {
					logger.Error(ctx, e, "app", appName)
				}
				return fmt.Errorf("validation failed (%d errors)", len(vout.Errors))
			}
			for _, w := range vout.Warnings {
				logger.Warn(ctx, w, "app", appName)
			}
			if len(vout.K8sObjects) == 0 {
				return fmt.Errorf("no Kubernetes objects generated for app %s", appName)
			}

			// Retrieve related cluster/provider/service for kubeconfig
			// (Direct repo usage instead of new usecase to minimize dependencies)
			clusterObj, err := appUC.Repos.Cluster.Get(ctx, target.ClusterID)
			if err != nil || clusterObj == nil {
				return fmt.Errorf("failed to get cluster %s: %w", target.ClusterID, err)
			}
			providerObj, err := appUC.Repos.Provider.Get(ctx, clusterObj.ProviderID)
			if err != nil || providerObj == nil {
				return fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
			}
			var serviceObj *model.Service
			if providerObj.ServiceID != "" {
				serviceObj, _ = appUC.Repos.Service.Get(ctx, providerObj.ServiceID)
			}

			// Instantiate provider driver to access kubeconfig
			factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
			if !ok {
				return fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
			}
			drv, err := factory(serviceObj, providerObj)
			if err != nil {
				return fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
			}
			kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
			if err != nil {
				return fmt.Errorf("failed to get cluster kubeconfig: %w", err)
			}

			kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
			if err != nil {
				return fmt.Errorf("failed to create kube client: %w", err)
			}

			// Ensure GVK is set on all objects before SSA; otherwise they are skipped as no-op.
			scheme := runtime.NewScheme()
			utilruntime.Must(appsv1.AddToScheme(scheme))
			utilruntime.Must(corev1.AddToScheme(scheme))
			utilruntime.Must(netv1.AddToScheme(scheme))
			for _, obj := range vout.K8sObjects {
				if obj == nil {
					continue
				}
				if gvk, _, err := scheme.ObjectKinds(obj); err == nil && len(gvk) > 0 {
					obj.GetObjectKind().SetGroupVersionKind(gvk[0])
				}
			}

			// Apply objects via server-side apply (SSA)
			if err := kcli.ServerSideApplyObjects(ctx, vout.K8sObjects, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
				return fmt.Errorf("apply objects failed: %w", err)
			}

			logger.Info(ctx, "deploy success", "app", appName)
			return nil
		},
	}
	return cmd
}
