package main

import (
	"bytes"
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
	yaml "gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				manifest, err := buildCleanManifest(out.K8sObjects)
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

			// Apply objects
			if err := applyK8sObjects(ctx, kcli, vout.K8sObjects, logger); err != nil {
				return fmt.Errorf("apply objects failed: %w", err)
			}

			logger.Info(ctx, "deploy success", "app", appName)
			return nil
		},
	}
	return cmd
}

// applyK8sObjects applies a slice of runtime.Objects using simple create-or-update semantics.

func applyK8sObjects(ctx context.Context, kc *kube.Client, objs []runtime.Object, logger logging.Logger) error {
	var errs []string
	apply := func(kind, name string,
		get func() (runtime.Object, error),
		create func() (runtime.Object, error),
		prepare func(existing runtime.Object) (runtime.Object, error),
		update func(runtime.Object) (runtime.Object, error),
		logUpdate bool,
	) {
		exist, err := get()
		if err != nil {
			if apierrors.IsNotFound(err) {
				if _, cerr := create(); cerr != nil {
					logger.Error(ctx, fmt.Sprintf("create error: %v", cerr), "kind", kind, "name", name, "op", "create")
					errs = append(errs, fmt.Sprintf("create %s %s: %v", kind, name, cerr))
					return
				}
				logger.Info(ctx, "created "+kind, "name", name)
				return
			}
			logger.Error(ctx, fmt.Sprintf("get error: %v", err), "kind", kind, "name", name, "op", "get")
			errs = append(errs, fmt.Sprintf("get %s %s: %v", kind, name, err))
			return
		}
		objForUpdate, err := prepare(exist)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("prepare error: %v", err), "kind", kind, "name", name, "op", "prepare")
			errs = append(errs, fmt.Sprintf("prepare %s %s: %v", kind, name, err))
			return
		}
		if _, err := update(objForUpdate); err != nil {
			logger.Error(ctx, fmt.Sprintf("update error: %v", err), "kind", kind, "name", name, "op", "update")
			errs = append(errs, fmt.Sprintf("update %s %s: %v", kind, name, err))
			return
		}
		if logUpdate {
			logger.Info(ctx, "updated "+kind, "name", name)
		}
	}

	for _, obj := range objs {
		switch o := obj.(type) {
		case *corev1.Namespace:
			nsClient := kc.Clientset.CoreV1().Namespaces()
			apply("namespace", o.Name,
				func() (runtime.Object, error) { return nsClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return nsClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) { // merge labels/annotations
					e := existing.(*corev1.Namespace)
					if e.Labels == nil {
						e.Labels = map[string]string{}
					}
					for k, v := range o.Labels {
						e.Labels[k] = v
					}
					if e.Annotations == nil {
						e.Annotations = map[string]string{}
					}
					for k, v := range o.Annotations {
						e.Annotations[k] = v
					}
					return e, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return nsClient.Update(ctx, ro.(*corev1.Namespace), metav1.UpdateOptions{})
				},
				true)
		case *corev1.PersistentVolume:
			pvClient := kc.Clientset.CoreV1().PersistentVolumes()
			apply("pv", o.Name,
				func() (runtime.Object, error) { return pvClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return pvClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) {
					e := existing.(*corev1.PersistentVolume)
					// no pre-check; rely on API server immutability validation
					o.ResourceVersion = e.ResourceVersion
					return o, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return pvClient.Update(ctx, ro.(*corev1.PersistentVolume), metav1.UpdateOptions{})
				},
				true)
		case *corev1.PersistentVolumeClaim:
			pvcClient := kc.Clientset.CoreV1().PersistentVolumeClaims(o.Namespace)
			apply("pvc", o.Name,
				func() (runtime.Object, error) { return pvcClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return pvcClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) {
					e := existing.(*corev1.PersistentVolumeClaim)
					// no pre-check; rely on API server immutability/validation errors
					o.ResourceVersion = e.ResourceVersion
					return o, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return pvcClient.Update(ctx, ro.(*corev1.PersistentVolumeClaim), metav1.UpdateOptions{})
				},
				true)
		case *appsv1.Deployment:
			depClient := kc.Clientset.AppsV1().Deployments(o.Namespace)
			apply("deployment", o.Name,
				func() (runtime.Object, error) { return depClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return depClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) {
					e := existing.(*appsv1.Deployment)
					o.ResourceVersion = e.ResourceVersion
					return o, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return depClient.Update(ctx, ro.(*appsv1.Deployment), metav1.UpdateOptions{})
				},
				true)
		case *corev1.Service:
			svcClient := kc.Clientset.CoreV1().Services(o.Namespace)
			apply("service", o.Name,
				func() (runtime.Object, error) { return svcClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return svcClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) {
					e := existing.(*corev1.Service)
					// preserve immutable fields
					if e.Spec.ClusterIP != "" {
						o.Spec.ClusterIP = e.Spec.ClusterIP
					}
					if len(e.Spec.ClusterIPs) > 0 {
						o.Spec.ClusterIPs = e.Spec.ClusterIPs
					}
					o.ResourceVersion = e.ResourceVersion
					return o, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return svcClient.Update(ctx, ro.(*corev1.Service), metav1.UpdateOptions{})
				},
				true)
		case *netv1.Ingress:
			ingClient := kc.Clientset.NetworkingV1().Ingresses(o.Namespace)
			apply("ingress", o.Name,
				func() (runtime.Object, error) { return ingClient.Get(ctx, o.Name, metav1.GetOptions{}) },
				func() (runtime.Object, error) { return ingClient.Create(ctx, o, metav1.CreateOptions{}) },
				func(existing runtime.Object) (runtime.Object, error) {
					e := existing.(*netv1.Ingress)
					o.ResourceVersion = e.ResourceVersion
					return o, nil
				},
				func(ro runtime.Object) (runtime.Object, error) {
					return ingClient.Update(ctx, ro.(*netv1.Ingress), metav1.UpdateOptions{})
				},
				true)
		default:
			continue
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

// pruneManifest takes a multi-document YAML (--- separators) and removes keys whose value is null or an empty map.
// Empty lists are preserved (they can be semantically meaningful). Entire documents that become empty are dropped.
// buildCleanManifest converts runtime.Objects to unstructured maps, prunes null/empty maps using reflection style traversal, then marshals as multi-doc YAML.
func buildCleanManifest(objs []runtime.Object) (string, error) {
	var buf bytes.Buffer
	for _, obj := range objs {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return "", err
		}
		pruneMap(m)
		// Drop metadata.creationTimestamp explicitly (often zero => null output)
		if meta, ok := m["metadata"].(map[string]interface{}); ok {
			delete(meta, "creationTimestamp")
			if len(meta) == 0 { // unlikely
				delete(m, "metadata")
			}
		}
		// Drop empty status
		if st, ok := m["status"].(map[string]interface{}); ok && len(st) == 0 {
			delete(m, "status")
		}
		var ybuf bytes.Buffer
		enc := yaml.NewEncoder(&ybuf)
		enc.SetIndent(2)
		if err := enc.Encode(m); err != nil {
			return "", err
		}
		_ = enc.Close()
		y := ybuf.Bytes()
		buf.WriteString("---\n")
		buf.Write(y)
		if len(y) == 0 || y[len(y)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	return buf.String(), nil
}

// pruneMap recursively removes keys with nil or empty map values.
func pruneMap(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, val := range x {
			cleaned := pruneMap(val)
			switch cv := cleaned.(type) {
			case nil:
				delete(x, k)
			case map[string]interface{}:
				if len(cv) == 0 {
					delete(x, k)
				} else {
					x[k] = cv
				}
			default:
				x[k] = cv
			}
		}
		return x
	case []interface{}:
		for i, it := range x {
			x[i] = pruneMap(it)
		}
		return x
	default:
		return x
	}
}
