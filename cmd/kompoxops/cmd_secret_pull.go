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

// newCmdSecretPull manages the image pull secret under the 'secret' group.
func newCmdSecretPull() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "pull",
		Short:              "Manage image pull (registry auth) Secret (<app>-<component>--pull)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	cmd.AddCommand(newCmdSecretPullSet(), newCmdSecretPullDelete())
	return cmd
}

func newCmdSecretPullSet() *cobra.Command {
	var filePath string
	var component string
	var dryRun bool
	cmd := &cobra.Command{
		Use:                "set",
		Short:              "Create or update image pull secret from docker config JSON",
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

			appID, err := resolveAppID(ctx, secUC.Repos.App, args)
			if err != nil {
				return err
			}

			data, rerr := os.ReadFile(filePath)
			if rerr != nil {
				return fmt.Errorf("read file: %w", rerr)
			}
			if component == "" {
				component = "app"
			}
			in := &secret.PullInput{AppID: appID, Operation: secret.PullOpSet, ComponentName: component, FilePath: filePath, FileContent: data, DryRun: dryRun}
			out, err := secUC.Pull(ctx, in)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Docker config JSON path (e.g. ~/.docker/config.json)")
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show result without applying changes")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func newCmdSecretPullDelete() *cobra.Command {
	var component string
	var dryRun bool
	cmd := &cobra.Command{
		Use:                "delete",
		Short:              "Delete image pull secret",
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

			appID, err := resolveAppID(ctx, secUC.Repos.App, args)
			if err != nil {
				return err
			}
			if component == "" {
				component = "app"
			}
			in := &secret.PullInput{AppID: appID, Operation: secret.PullOpDelete, ComponentName: component, DryRun: dryRun}
			out, err := secUC.Pull(ctx, in)
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show result without applying changes")
	return cmd
}
