package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/internal/logging"
	"github.com/yaegashi/kompoxops/usecase/app"
	yaml "gopkg.in/yaml.v3"
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
	cmd.AddCommand(newCmdAppValidate())
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
