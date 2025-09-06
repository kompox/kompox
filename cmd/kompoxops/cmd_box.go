package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/terminal"
	boxuc "github.com/kompox/kompox/usecase/box"
	"github.com/spf13/cobra"
)

const defaultBoxImage = "ghcr.io/kompox/kompox/box"

var flagBoxAppName string

func newCmdBox() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "box",
		Short:              "Manage Kompox Box in app namespace",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	cmd.PersistentFlags().StringVarP(&flagBoxAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.AddCommand(newCmdBoxDeploy(), newCmdBoxDestroy(), newCmdBoxStatus(), newCmdBoxExec(), newCmdBoxSsh(), newCmdBoxSCP())
	return cmd
}

// Resolve app name from flag or config file. No positional args supported.
func getBoxAppName(_ *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return "", fmt.Errorf("positional app name is not supported; use --app-name")
	}
	if flagBoxAppName != "" {
		return flagBoxAppName, nil
	}
	if configRoot != nil && len(configRoot.App.Name) > 0 {
		return configRoot.App.Name, nil
	}
	return "", fmt.Errorf("app name not specified; use --app-name or set app.name in kompoxops.yml")
}

// resolveAppIDByNameUsingBoxUC lists apps via the same repositories used by boxUC to avoid ID mismatches.
func resolveAppIDByNameUsingBoxUC(ctx context.Context, boxUC *boxuc.UseCase, name string) (string, error) {
	apps, err := boxUC.Repos.App.List(ctx)
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

func newCmdBoxDeploy() *cobra.Command {
	var image string
	var volumes []string
	var command []string
	var argsFlag []string
	var sshPubkeyFile string
	var alwaysPull bool
	cmd := &cobra.Command{
		Use:                "deploy",
		Short:              "Deploy Kompox Box (Deployment) with PV/PVC mounts",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			logger := logging.FromContext(ctx)

			appName, err := getBoxAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingBoxUC(ctx, boxUC, appName)
			if err != nil {
				return err
			}

			// Determine and read SSH public key file
			if sshPubkeyFile == "" {
				// Try common default public key locations in order
				home := os.Getenv("HOME")
				// On Windows, HOME may be empty; try USERPROFILE or HOMEDRIVE+HOMEPATH
				if home == "" {
					home = os.Getenv("USERPROFILE")
				}
				if home == "" {
					hd := os.Getenv("HOMEDRIVE")
					hp := os.Getenv("HOMEPATH")
					if hd != "" || hp != "" {
						home = filepath.Join(hd, hp)
					}
				}
				candidates := []string{
					filepath.Join(home, ".ssh", "id_rsa.pub"),
					filepath.Join(home, ".ssh", "id_ecdsa.pub"),
					filepath.Join(home, ".ssh", "id_ed25519.pub"),
				}
				for _, p := range candidates {
					if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
						sshPubkeyFile = p
						break
					}
				}
				if sshPubkeyFile == "" {
					return fmt.Errorf("SSH public key not found: specify --ssh-pubkey or place one of %v", candidates)
				}
			}

			// Read the found SSH public key file
			logger.Info(ctx, "reading SSH public key", "path", sshPubkeyFile)
			sshPubkey, err := os.ReadFile(sshPubkeyFile)
			if err != nil {
				return fmt.Errorf("failed to read SSH public key file %s: %w", sshPubkeyFile, err)
			}

			if _, err := boxUC.Deploy(ctx, &boxuc.DeployInput{
				AppID:      appID,
				Image:      image,
				Command:    command,
				Args:       argsFlag,
				Volumes:    volumes,
				AlwaysPull: alwaysPull,
				SSHPubkey:  sshPubkey,
			}); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", defaultBoxImage, "Container image for box")
	cmd.Flags().StringArrayVarP(&command, "command", "c", nil, "Container entrypoint (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&argsFlag, "args", "a", nil, "Container arguments (repeat to pass multiple tokens)")
	cmd.Flags().StringArrayVarP(&volumes, "volume", "V", nil, "Volume mount spec 'volName:diskName:/mount/path' (repeat, optional)")
	cmd.Flags().StringVar(&sshPubkeyFile, "ssh-pubkey", "", "SSH public key file path (optional)")
	cmd.Flags().BoolVar(&alwaysPull, "always-pull", false, "Always pull container image (ImagePullPolicy: Always)")
	return cmd
}

func newCmdBoxDestroy() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "destroy",
		Short:              "Destroy Kompox Box workload (keep PV/PVC)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			appName, err := getBoxAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingBoxUC(ctx, boxUC, appName)
			if err != nil {
				return err
			}
			if _, err := boxUC.Destroy(ctx, &boxuc.DestroyInput{AppID: appID}); err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}

func newCmdBoxStatus() *cobra.Command {
	return &cobra.Command{
		Use:                "status",
		Short:              "Show Kompox Box status",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			appName, err := getBoxAppName(cmd, args)
			if err != nil {
				return err
			}
			appID, err := resolveAppIDByNameUsingBoxUC(ctx, boxUC, appName)
			if err != nil {
				return err
			}
			out, err := boxUC.Status(ctx, &boxuc.StatusInput{AppID: appID})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}

func newCmdBoxExec() *cobra.Command {
	var stdin bool
	var tty bool
	var escape string
	cmd := &cobra.Command{
		Use:                "exec -- <command> [args...]",
		Short:              "Exec into Kompox Box pod",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// localize klog suppression only for exec
			terminal.QuietKlog()
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			// Exec can be long-running/interactive; don't force timeout here.
			ctx := cmd.Context()
			logger := logging.FromContext(ctx)

			appName, err := getBoxAppName(cmd, nil)
			if err != nil {
				return err
			}
			// Resolve app ID using a short-lived context
			{
				rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				appID, err := resolveAppIDByNameUsingBoxUC(rctx, boxUC, appName)
				if err != nil {
					return err
				}
				if len(args) == 0 {
					return fmt.Errorf("command is required after --")
				}
				_, err = boxUC.Exec(ctx, &boxuc.ExecInput{AppID: appID, Command: args, Stdin: stdin, TTY: tty, Escape: escape})
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

func newCmdBoxSsh() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "ssh -- [ssh args...]",
		Short:              "SSH into Kompox Box pod via port forwarding",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			logger := logging.FromContext(ctx)

			appName, err := getBoxAppName(cmd, nil)
			if err != nil {
				return err
			}
			// Resolve app ID using a short-lived context
			{
				rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				appID, err := resolveAppIDByNameUsingBoxUC(rctx, boxUC, appName)
				if err != nil {
					return err
				}

				_, err = boxUC.SSH(ctx, &boxuc.SSHInput{AppID: appID, SSHArgs: args})
				if err != nil {
					logger.Error(ctx, "SSH failed", "error", err)
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

func newCmdBoxSCP() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "scp -- [scp args...]",
		Aliases:            []string{"cp"},
		Short:              "Transfer files to/from Kompox Box pod via port forwarding",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			boxUC, err := buildBoxUseCase(cmd)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			logger := logging.FromContext(ctx)

			appName, err := getBoxAppName(cmd, nil)
			if err != nil {
				return err
			}
			// Resolve app ID using a short-lived context
			{
				rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()
				appID, err := resolveAppIDByNameUsingBoxUC(rctx, boxUC, appName)
				if err != nil {
					return err
				}

				_, err = boxUC.SCP(ctx, &boxuc.SCPInput{AppID: appID, SCPArgs: args})
				if err != nil {
					logger.Error(ctx, "SCP failed", "error", err)
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
