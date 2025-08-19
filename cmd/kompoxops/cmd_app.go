package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/internal/logging"
	"github.com/yaegashi/kompoxops/usecase/app"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	serjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
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
			// Find app by name
			apps, err := appUC.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var target *string
			for _, a := range apps {
				if a.Name == appName {
					id := a.ID
					target = &id
					break
				}
			}
			if target == nil {
				return fmt.Errorf("app %s not found", appName)
			}
			out, err := appUC.Validate(ctx, app.ValidateInput{ID: *target})
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
				ser := serjson.NewSerializerWithOptions(
					serjson.DefaultMetaFactory, scheme, scheme,
					serjson.SerializerOptions{Yaml: true, Pretty: true, Strict: true},
				)
				var buf bytes.Buffer
				for _, obj := range out.K8sObjects {
					// Ensure GVK is populated so apiVersion/kind appear in output
					if gvk, _, err := scheme.ObjectKinds(obj); err == nil && len(gvk) > 0 {
						obj.GetObjectKind().SetGroupVersionKind(gvk[0])
					}
					io.WriteString(&buf, "---\n")
					if err := ser.Encode(obj, &buf); err != nil {
						return fmt.Errorf("failed to encode kubernetes object: %w", err)
					}
				}
				if outManifestPath == "-" {
					fmt.Fprint(cmd.OutOrStdout(), buf.String())
				} else if err := os.WriteFile(outManifestPath, buf.Bytes(), 0o644); err != nil {
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
