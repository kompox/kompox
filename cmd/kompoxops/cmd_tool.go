package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kompox/kompox/internal/logging"
	tooluc "github.com/kompox/kompox/usecase/tool"
	"github.com/spf13/cobra"
)

var flagToolAppName string

func newCmdTool() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "tool",
		Short:              "Manage maintenance runner (tool) in app namespace",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	cmd.PersistentFlags().StringVarP(&flagToolAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.AddCommand(newCmdToolDeploy(), newCmdToolDestroy(), newCmdToolStatus(), newCmdToolExec())
	return cmd
}

// Resolve app name from flag or config file. No positional args supported.
func getToolAppName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("positional app name is not supported; use --app-name")
	}
	if flagToolAppName != "" {
		return flagToolAppName, nil
	}
	if configRoot != nil && len(configRoot.App.Name) > 0 {
		return configRoot.App.Name, nil
	}
	return "", fmt.Errorf("app name not specified; use --app-name or set app.name in kompoxops.yml")
}

// resolveAppIDByNameUsingToolUC lists apps via the same repositories used by toolUC to avoid ID mismatches.
func resolveAppIDByNameUsingToolUC(ctx context.Context, toolUC *tooluc.UseCase, name string) (string, error) {
	apps, err := toolUC.Repos.App.List(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range apps {
		if a.Name == name {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("app %s not found", name)
}

func newCmdToolDeploy() *cobra.Command {
	var image string
	var volumes []string
	var command []string
	var argsFlag []string
	cmd := &cobra.Command{
		Use:                "deploy",
		Short:              "Deploy tool runner (Deployment) with PV/PVC mounts",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolUC, err := buildToolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appName, err := getToolAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingToolUC(ctx, toolUC, appName)
			if err != nil {
				return err
			}
			if _, err := toolUC.Deploy(ctx, &tooluc.DeployInput{AppID: appID, Image: image, Command: command, Args: argsFlag, Volumes: volumes}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "busybox", "Container image for runner (default: busybox)")
	cmd.Flags().StringArrayVarP(&command, "command", "c", nil, "Container entrypoint (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&argsFlag, "args", "a", nil, "Container arguments (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&volumes, "volume", "V", nil, "Volume mount spec 'volName:diskName:/mount/path' (repeat, optional)")
	return cmd
}

func newCmdToolDestroy() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "destroy",
		Short:              "Destroy tool runner workload (keep PV/PVC)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolUC, err := buildToolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appName, err := getToolAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingToolUC(ctx, toolUC, appName)
			if err != nil {
				return err
			}
			if _, err := toolUC.Destroy(ctx, &tooluc.DestroyInput{AppID: appID}); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}

func newCmdToolStatus() *cobra.Command {
	return &cobra.Command{
		Use:                "status",
		Short:              "Show tool runner status",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			toolUC, err := buildToolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			appName, err := getToolAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingToolUC(ctx, toolUC, appName)
			if err != nil {
				return err
			}
			out, err := toolUC.Status(ctx, &tooluc.StatusInput{AppID: appID})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}

func newCmdToolExec() *cobra.Command {
	var stdin bool
	var tty bool
	var escape string
	cmd := &cobra.Command{
		Use:                "exec -- <command> [args...]",
		Short:              "Exec into tool runner pod",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// localize klog suppression only for exec
			quietKlog()
			toolUC, err := buildToolUseCase(cmd)
			if err != nil {
				return err
			}
			// Exec can be long-running/interactive; don't force timeout here.
			ctx := cmd.Context()
			logger := logging.FromContext(ctx)

			appName, err := getToolAppName(cmd, nil)
			if err != nil {
				return err
			}
			// Resolve app ID using a short-lived context
			{
				rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				appID, err := resolveAppIDByNameUsingToolUC(rctx, toolUC, appName)
				if err != nil {
					return err
				}
				if len(args) == 0 {
					return fmt.Errorf("command is required after --")
				}
				_, err = toolUC.Exec(ctx, &tooluc.ExecInput{AppID: appID, Command: args, Stdin: stdin, TTY: tty, Escape: escape})
				if err != nil {
					logger.Error(ctx, "exec failed", "error", err)
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&stdin, "stdin", "i", false, "Pass stdin to the command")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a TTY")
	cmd.Flags().StringVarP(&escape, "escape", "e", "~.", "Escape sequence to detach (e.g. '~.'); set 'none' to disable")
	return cmd
}
