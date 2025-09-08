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

// newCmdSnapshot returns the root snapshot command.
func newCmdSnapshot() *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Short: "Manage volume snapshots", SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true, RunE: func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") }}
	// Reuse common app name flag variable defined in cmd_disk.go
	cmd.PersistentFlags().StringVarP(&flagVolumeAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required)")
	// Per-subcommand flags: disk-name and snapshot-name
	cmd.AddCommand(newCmdSnapshotList(), newCmdSnapshotCreate(), newCmdSnapshotDelete(), newCmdSnapshotRestore())
	return cmd
}

func getSnapshotAppName(cmd *cobra.Command) (string, error) {
	// Reuse disk helper to keep a single source of truth
	return getDiskAppName(cmd)
}

func resolveAppIDByNameForSnapshot(ctx context.Context, u *vuc.UseCase, appName string) (string, error) {
	// Reuse the same lookup as disk commands
	return resolveAppIDByName(ctx, u, appName)
}

func newCmdSnapshotList() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List snapshots", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		appName, err := getSnapshotAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		appID, err := resolveAppIDByNameForSnapshot(ctx, u, appName)
		if err != nil {
			return err
		}
		out, err := u.SnapshotList(ctx, &vuc.SnapshotListInput{AppID: appID, VolumeName: volName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Items)
	}}
	return cmd
}

func newCmdSnapshotCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create snapshot from a disk", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getSnapshotAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		// disk-name is provided as a flag for create
		diskName, _ := cmd.Flags().GetString("disk-name")
		if diskName == "" {
			return fmt.Errorf("--disk-name required")
		}
		appID, err := resolveAppIDByNameForSnapshot(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "create snapshot", "app", appName, "volume", volName, "disk", diskName)
		out, err := u.SnapshotCreate(ctx, &vuc.SnapshotCreateInput{AppID: appID, VolumeName: volName, DiskName: diskName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Snapshot)
	}}
	cmd.Flags().StringP("disk-name", "D", "", "Disk name (required)")
	return cmd
}

func newCmdSnapshotDelete() *cobra.Command {
	cmd := &cobra.Command{Use: "delete", Short: "Delete snapshot", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getSnapshotAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		snapName, _ := cmd.Flags().GetString("snapshot-name")
		if snapName == "" {
			return fmt.Errorf("--snapshot-name required")
		}
		appID, err := resolveAppIDByNameForSnapshot(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "delete snapshot", "app", appName, "volume", volName, "snapshot", snapName)
		if _, err := u.SnapshotDelete(ctx, &vuc.SnapshotDeleteInput{AppID: appID, VolumeName: volName, SnapshotName: snapName}); err != nil {
			return err
		}
		return nil
	}}
	cmd.Flags().StringP("snapshot-name", "S", "", "Snapshot name (required)")
	return cmd
}

func newCmdSnapshotRestore() *cobra.Command {
	cmd := &cobra.Command{Use: "restore", Short: "Restore snapshot into a new disk", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getSnapshotAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		snapName, _ := cmd.Flags().GetString("snapshot-name")
		if snapName == "" {
			return fmt.Errorf("--snapshot-name required")
		}
		appID, err := resolveAppIDByNameForSnapshot(ctx, u, appName)
		if err != nil {
			return err
		}

		// Parse zone and options flags
		zone, _ := cmd.Flags().GetString("zone")
		optionsStr, _ := cmd.Flags().GetString("options")

		input := &vuc.SnapshotRestoreInput{
			AppID:        appID,
			VolumeName:   volName,
			SnapshotName: snapName,
			Zone:         zone,
		}

		// Parse JSON options if provided
		if optionsStr != "" {
			var options map[string]any
			if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
				return fmt.Errorf("invalid JSON in --options: %w", err)
			}
			input.Options = options
		}

		logger.Info(ctx, "restore snapshot", "app", appName, "volume", volName, "snapshot", snapName)
		out, err := u.SnapshotRestore(ctx, input)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Disk)
	}}
	cmd.Flags().StringP("snapshot-name", "S", "", "Snapshot name (required)")
	cmd.Flags().StringP("zone", "Z", "", "Override deployment zone")
	cmd.Flags().StringP("options", "O", "", "Override volume options (JSON)")
	return cmd
}
