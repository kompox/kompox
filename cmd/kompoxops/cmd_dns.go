package main

import (
	"context"
	"fmt"
	"time"

	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/usecase/dns"
	"github.com/spf13/cobra"
)

func newCmdDNS() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "dns",
		Short:              "Manage DNS records for cluster ingress",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	// Persistent flag shared across subcommands (same as app command)
	cmd.PersistentFlags().StringVarP(&flagAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.AddCommand(newCmdDNSDeploy(), newCmdDNSDestroy())
	return cmd
}

func newCmdDNSDeploy() *cobra.Command {
	var strict bool
	var dryRun bool
	var component string

	cmd := &cobra.Command{
		Use:                "deploy",
		Short:              "Deploy DNS records for cluster ingress endpoints",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dnsUC, err := buildDNSUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			// Resolve app name using shared getAppName function
			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}

			// Find app by name
			apps, err := dnsUC.Repos.App.List(ctx)
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

			if component == "" {
				component = "app"
			}

			logger.Info(ctx, "dns deploy start", "app", appName, "component", component, "strict", strict, "dry_run", dryRun)

			out, err := dnsUC.Deploy(ctx, &dns.DeployInput{
				AppID:         appID,
				ComponentName: component,
				Strict:        strict,
				DryRun:        dryRun,
			})
			if err != nil {
				return fmt.Errorf("failed to deploy DNS: %w", err)
			}

			// Output results
			if len(out.Applied) == 0 {
				logger.Info(ctx, "no DNS records to apply")
				return nil
			}

			for _, r := range out.Applied {
				switch r.Action {
				case "planned":
					logger.Info(ctx, "would apply DNS record", "fqdn", r.FQDN, "type", r.Type, "message", r.Message)
				case "updated", "created":
					logger.Info(ctx, "applied DNS record", "fqdn", r.FQDN, "type", r.Type, "message", r.Message)
				case "skipped":
					logger.Warn(ctx, "skipped DNS record", "fqdn", r.FQDN, "message", r.Message)
				case "failed":
					logger.Error(ctx, "failed to apply DNS record", "fqdn", r.FQDN, "type", r.Type, "error", r.Message)
				}
			}

			logger.Info(ctx, "dns deploy complete")
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat DNS update failures as errors")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be changed without applying")
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")

	return cmd
}

func newCmdDNSDestroy() *cobra.Command {
	var strict bool
	var dryRun bool
	var component string

	cmd := &cobra.Command{
		Use:                "destroy",
		Short:              "Destroy DNS records for cluster ingress endpoints",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dnsUC, err := buildDNSUseCase(cmd)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			logger := logging.FromContext(ctx)

			// Resolve app name using shared getAppName function
			appName, err := getAppName(cmd, args)
			if err != nil {
				return err
			}

			// Find app by name
			apps, err := dnsUC.Repos.App.List(ctx)
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

			if component == "" {
				component = "app"
			}

			logger.Info(ctx, "dns destroy start", "app", appName, "component", component, "strict", strict, "dry_run", dryRun)

			out, err := dnsUC.Destroy(ctx, &dns.DestroyInput{
				AppID:         appID,
				ComponentName: component,
				Strict:        strict,
				DryRun:        dryRun,
			})
			if err != nil {
				return fmt.Errorf("failed to destroy DNS: %w", err)
			}

			// Output results
			if len(out.Deleted) == 0 {
				logger.Info(ctx, "no DNS records to delete")
				return nil
			}

			for _, r := range out.Deleted {
				switch r.Action {
				case "planned":
					logger.Info(ctx, "would delete DNS record", "fqdn", r.FQDN, "type", r.Type, "message", r.Message)
				case "deleted":
					logger.Info(ctx, "deleted DNS record", "fqdn", r.FQDN, "type", r.Type, "message", r.Message)
				case "skipped":
					logger.Warn(ctx, "skipped DNS record", "fqdn", r.FQDN, "message", r.Message)
				case "failed":
					logger.Error(ctx, "failed to delete DNS record", "fqdn", r.FQDN, "type", r.Type, "error", r.Message)
				}
			}

			logger.Info(ctx, "dns destroy complete")
			return nil
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat DNS update failures as errors")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be changed without applying")
	cmd.Flags().StringVarP(&component, "component", "C", "app", "Component name (default: app)")

	return cmd
}
