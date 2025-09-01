package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/internal/logging"
	vuc "github.com/yaegashi/kompoxops/usecase/volume"
)

var flagVolumeAppName string

func newCmdDisk() *cobra.Command {
	cmd := &cobra.Command{Use: "disk", Short: "Manage app disks", SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true, RunE: func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") }}
	cmd.PersistentFlags().StringVarP(&flagVolumeAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required for list/create/assign/delete)")
	cmd.PersistentFlags().StringP("disk-name", "D", "", "Disk name (required for assign/delete)")
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
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "create volume instance start", "app", appName, "volume", volName)
		out, err := u.DiskCreate(ctx, &vuc.DiskCreateInput{AppID: appID, VolumeName: volName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Disk)
	}}
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
		diskName, _ := cmd.Flags().GetString("disk-name")
		if diskName == "" {
			return fmt.Errorf("--disk-name required")
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
		diskName, _ := cmd.Flags().GetString("disk-name")
		if diskName == "" {
			return fmt.Errorf("--disk-name required")
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
