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

var flagVolumeSnapshotName string

// newCmdSnapshot returns the root snapshot command.
func newCmdSnapshot() *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Short: "Manage volume snapshots", SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true, RunE: func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") }}
	// Reuse common app name flag variable defined in cmd_disk.go
	cmd.PersistentFlags().StringVarP(&flagVolumeAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required)")
	cmd.PersistentFlags().StringVarP(&flagVolumeSnapshotName, "name", "N", "", "Snapshot name (optional for create; required for delete)")
	cmd.PersistentFlags().StringVar(&flagVolumeSnapshotName, "snap-name", "", "Snapshot name (alias of --name)")
	// Per-subcommand flags: disk-name and snapshot-name
	cmd.AddCommand(newCmdSnapshotList(), newCmdSnapshotCreate(), newCmdSnapshotDelete())
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
	cmd := &cobra.Command{Use: "create", Short: "Create snapshot", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
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
		snapshotName := flagVolumeSnapshotName
		source, _ := cmd.Flags().GetString("source")
		appID, err := resolveAppIDByNameForSnapshot(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "create snapshot", "app", appName, "volume", volName, "source", source, "name", snapshotName)
		out, err := u.SnapshotCreate(ctx, &vuc.SnapshotCreateInput{AppID: appID, VolumeName: volName, SnapshotName: snapshotName, Source: source})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Snapshot)
	}}
	cmd.Flags().StringP("source", "S", "", "Source identifier for snapshot creation (forwarded to provider driver)")
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
		snapName := flagVolumeSnapshotName
		if snapName == "" {
			return fmt.Errorf("--name (or --snap-name) required")
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
	return cmd
}
