package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/terminal"
	tuc "github.com/kompox/kompox/usecase/tool"
	"github.com/spf13/cobra"
)

const defaultToolImage = "ghcr.io/kompox/kompox/kompoxops:main"

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
	cmd.AddCommand(newCmdToolDeploy(), newCmdToolDestroy(), newCmdToolStatus(), newCmdToolExec(), newCmdToolRsync())
	return cmd
}

// kompoxops tool rsync vol:/remote/path /local/path などの形式でrsyncを実行
func newCmdToolRsync() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rsync -- [rsync options...] <source> <destination>",
		Short: "Rsync between local and tool runner vol:/...",
		Long: `Rsync files between local filesystem and tool runner volumes.

Examples:
  kompoxops tool rsync -- vol:/data /local/backup
  kompoxops tool rsync -- /local/data vol:/backup
  kompoxops tool rsync -- -av --delete /local/data/ vol:/data/
  kompoxops tool rsync -- vol:
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolUC, err := buildToolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			appName, err := getToolAppName(cmd, nil)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingToolUC(ctx, toolUC, appName)
			if err != nil {
				return err
			}

			// Parse arguments to separate rsync options from source/destination
			// All arguments are passed directly as RsyncArgs for vol: path transformation
			if len(args) == 0 {
				return fmt.Errorf("rsync arguments required")
			}

			// Validate that at least one argument contains vol: prefix
			hasVolPath := false
			for _, arg := range args {
				if strings.HasPrefix(arg, "vol:") {
					hasVolPath = true
					break
				}
			}
			if !hasVolPath {
				return fmt.Errorf("at least one argument must start with vol:")
			}

			in := &tuc.RsyncInput{
				AppID:     appID,
				RsyncArgs: args,
			}
			out, err := toolUC.Rsync(ctx, in)
			if err != nil {
				if out != nil && out.Stderr != "" {
					fmt.Fprint(cmd.ErrOrStderr(), out.Stderr)
				}
				return fmt.Errorf("rsync failed: %w", err)
			}
			if out != nil && out.Stdout != "" {
				fmt.Fprint(cmd.OutOrStdout(), out.Stdout)
			}
			return nil
		},
	}
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
func resolveAppIDByNameUsingToolUC(ctx context.Context, toolUC *tuc.UseCase, name string) (string, error) {
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
	var alwaysPull bool
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
			if _, err := toolUC.Deploy(ctx, &tuc.DeployInput{
				AppID:      appID,
				Image:      image,
				Command:    command,
				Args:       argsFlag,
				Volumes:    volumes,
				AlwaysPull: alwaysPull,
			}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", defaultToolImage, "Container image for runner")
	cmd.Flags().StringArrayVarP(&command, "command", "c", nil, "Container entrypoint (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&argsFlag, "args", "a", nil, "Container arguments (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&volumes, "volume", "V", nil, "Volume mount spec 'volName:diskName:/mount/path' (repeat, optional)")
	cmd.Flags().BoolVar(&alwaysPull, "always-pull", false, "Always pull container image (ImagePullPolicy: Always)")
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
			if _, err := toolUC.Destroy(ctx, &tuc.DestroyInput{AppID: appID}); err != nil {
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
			out, err := toolUC.Status(ctx, &tuc.StatusInput{AppID: appID})
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
			terminal.QuietKlog()
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
				_, err = toolUC.Exec(ctx, &tuc.ExecInput{AppID: appID, Command: args, Stdin: stdin, TTY: tty, Escape: escape})
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
