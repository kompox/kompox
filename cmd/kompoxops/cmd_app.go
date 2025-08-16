package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/internal/logging"
	"github.com/yaegashi/kompoxops/usecase/app"
)

func newCmdApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "app",
		Short:              "Manage apps",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	cmd.AddCommand(newCmdAppValidate())
	return cmd
}

// getAppName returns the app name from args if present, otherwise from loaded configuration file.
func getAppName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if configRoot != nil && len(configRoot.App.Name) > 0 {
		return configRoot.App.Name, nil
	}
	return "", fmt.Errorf("app name not specified and no default available; provide app-name or use --db-url=file:/path/to/kompoxops.yml")
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
		Use:                "validate [app-name]",
		Short:              "Validate app compose definition",
		Args:               cobra.MaximumNArgs(1),
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
				return err
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
			if outManifestPath != "" && out.K8sManifest != "" {
				yamlDocs := normalizeYAMLDocs(out.K8sManifest)
				if outManifestPath == "-" {
					fmt.Fprint(cmd.OutOrStdout(), yamlDocs)
				} else if err := os.WriteFile(outManifestPath, []byte(yamlDocs), 0o644); err != nil {
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
