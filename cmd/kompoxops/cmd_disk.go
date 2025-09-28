package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kompox/kompox/internal/logging"
	vuc "github.com/kompox/kompox/usecase/volume"
	"github.com/spf13/cobra"
)

var (
	flagVolumeAppName  string
	flagVolumeDiskName string
)

func newCmdDisk() *cobra.Command {
	cmd := &cobra.Command{Use: "disk", Short: "Manage app disks", SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true, RunE: func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") }}
	cmd.PersistentFlags().StringVarP(&flagVolumeAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required for list/create/assign/delete)")
	cmd.PersistentFlags().StringVarP(&flagVolumeDiskName, "name", "N", "", "Disk name (optional for create; required for assign/delete)")
	cmd.PersistentFlags().StringVar(&flagVolumeDiskName, "disk-name", "", "Disk name (alias of --name)")
	cmd.AddCommand(newCmdDiskList(), newCmdDiskCreate(), newCmdDiskAssign(), newCmdDiskDelete())
	return cmd
}

func getDiskAppName(_ *cobra.Command) (string, error) {
	if flagVolumeAppName != "" {
		return flagVolumeAppName, nil
	}
	if configRoot != nil && configRoot.App.Name != "" {
		return configRoot.App.Name, nil
	}
	return "", fmt.Errorf("app name not specified; use --app-name or set app.name in kompoxops.yml")
}

func resolveAppIDByName(ctx context.Context, u *vuc.UseCase, appName string) (string, error) {
	apps, err := u.Repos.App.List(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range apps {
		if a.Name == appName {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("app %s not found", appName)
}

func newCmdDiskList() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List volume instances", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		appName, err := getDiskAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		out, err := u.DiskList(ctx, &vuc.DiskListInput{AppID: appID, VolumeName: volName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Items)
	}}
	return cmd
}

func newCmdDiskCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getDiskAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		bootstrap, _ := cmd.Flags().GetBool("bootstrap")
		diskName := flagVolumeDiskName
		if bootstrap && volName != "" {
			return fmt.Errorf("--vol-name must not be specified with --bootstrap")
		}
		if !bootstrap && volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		if bootstrap && diskName != "" {
			return fmt.Errorf("--name/--disk-name must not be specified with --bootstrap")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}

		// Parse zone, options, and source flags
		zone, _ := cmd.Flags().GetString("zone")
		optionsStr, _ := cmd.Flags().GetString("options")
		source, _ := cmd.Flags().GetString("source")
		var options map[string]any
		if optionsStr != "" {
			if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
				return fmt.Errorf("invalid JSON in --options: %w", err)
			}
		}

		if bootstrap {
			logger.Info(ctx, "bootstrap volume disks start", "app", appName)
			bout, err := u.DiskCreateBootstrap(ctx, &vuc.DiskCreateBootstrapInput{AppID: appID, Zone: zone, Options: options})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			// Flatten created disks to a simple array; if skipped return empty array.
			disks := make([]any, 0, len(bout.Created))
			for _, c := range bout.Created {
				if c != nil && c.Disk != nil {
					disks = append(disks, c.Disk)
				}
			}
			return enc.Encode(disks)
		}

		input := &vuc.DiskCreateInput{AppID: appID, VolumeName: volName, DiskName: diskName, Zone: zone, Options: options, Source: source}
		logger.Info(ctx, "create volume instance start", "app", appName, "volume", volName, "name", diskName, "source", source)
		out, err := u.DiskCreate(ctx, input)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Disk)
	}}
	cmd.Flags().StringP("zone", "Z", "", "Override deployment zone")
	cmd.Flags().StringP("options", "O", "", "Override volume options (JSON)")
	cmd.Flags().StringP("source", "S", "", "Source for disk creation (format depends on provider driver)")
	cmd.Flags().Bool("bootstrap", false, "Create one assigned disk per app volume if none are assigned (ignore when already initialized)")
	return cmd
}

func newCmdDiskAssign() *cobra.Command {
	cmd := &cobra.Command{Use: "assign", Short: "Assign volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getDiskAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		diskName := flagVolumeDiskName
		if diskName == "" {
			return fmt.Errorf("--name (or --disk-name) required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "assign volume disk", "app", appName, "volume", volName, "disk", diskName)
		if _, err := u.DiskAssign(ctx, &vuc.DiskAssignInput{AppID: appID, VolumeName: volName, DiskName: diskName}); err != nil {
			return err
		}
		return nil
	}}
	return cmd
}

func newCmdDiskDelete() *cobra.Command {
	cmd := &cobra.Command{Use: "delete", Short: "Delete volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getDiskAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		diskName := flagVolumeDiskName
		if diskName == "" {
			return fmt.Errorf("--name (or --disk-name) required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "delete volume disk", "app", appName, "volume", volName, "disk", diskName)
		if _, err := u.DiskDelete(ctx, &vuc.DiskDeleteInput{AppID: appID, VolumeName: volName, DiskName: diskName}); err != nil {
			return err
		}
		return nil
	}}
	return cmd
}
