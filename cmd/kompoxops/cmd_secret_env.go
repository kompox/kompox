package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/kompox/kompox/usecase/secret"
	"github.com/spf13/cobra"
)

// newCmdSecretEnv mirrors previous app env command under the new 'secret' grouping.
func newCmdSecretEnv() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "env",
		Short:              "Manage override environment variable Secret (<app>-<component>-<container>-override)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	cmd.AddCommand(newCmdSecretEnvSet(), newCmdSecretEnvDelete())
	return cmd
}

func newCmdSecretEnvSet() *cobra.Command {
	var filePath string
	var service string
	var component string
	var dryRun bool
	cmd := &cobra.Command{
		Use:                "set",
		Short:              "Create or update override env Secret from file",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			secUC, err := buildSecretUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			apps, err := secUC.Repos.App.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var appID string
			for _, a := range apps {
				if a != nil && a.Name == appName {
					appID = a.ID
					break
				}
			}
			if appID == "" {
				return fmt.Errorf("app %s not found", appName)
			}
			if service == "" {
				return fmt.Errorf("--service is required")
			}
			data, rerr := os.ReadFile(filePath)
			if rerr != nil {
				return fmt.Errorf("read file: %w", rerr)
			}
			if component == "" { // default
				component = "app"
			}
			in := &secret.EnvInput{AppID: appID, Operation: secret.EnvOpSet, ComponentName: component, ContainerName: service, FilePath: filePath, FileContent: data, DryRun: dryRun}
			out, err := secUC.Env(ctx, in)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Env file path (.env, .json, .yaml)")
	cmd.Flags().StringVarP(&service, "service", "S", "", "Service (compose) name (required)")
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show result without applying changes")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newCmdSecretEnvDelete() *cobra.Command {
	var service string
	var component string
	var dryRun bool
	cmd := &cobra.Command{
		Use:                "delete",
		Short:              "Delete override env Secret",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			secUC, err := buildSecretUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}
			apps, err := secUC.Repos.App.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list apps: %w", err)
			}
			var appID string
			for _, a := range apps {
				if a != nil && a.Name == appName {
					appID = a.ID
					break
				}
			}
			if appID == "" {
				return fmt.Errorf("app %s not found", appName)
			}
			if service == "" {
				return fmt.Errorf("--service is required")
			}
			if component == "" {
				component = "app"
			}
			in := &secret.EnvInput{AppID: appID, Operation: secret.EnvOpDelete, ComponentName: component, ContainerName: service, DryRun: dryRun}
			out, err := secUC.Env(ctx, in)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVarP(&service, "service", "S", "", "Service (compose) name (required)")
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show result without applying changes")
	return cmd
}
