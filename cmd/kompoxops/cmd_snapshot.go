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
	cmd.PersistentFlags().StringVarP(&flagAppID, "app-id", "A", "", "App ID (FQN: ws/prv/cls/app)")
	cmd.PersistentFlags().StringVar(&flagAppName, "app-name", "", "App name (backward compatibility, use --app-id)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required)")
	cmd.PersistentFlags().StringVarP(&flagVolumeSnapshotName, "name", "N", "", "Snapshot name (optional for create; required for delete)")
	cmd.PersistentFlags().StringVar(&flagVolumeSnapshotName, "snap-name", "", "Snapshot name (alias of --name)")
	// Per-subcommand flags: disk-name and snapshot-name
	cmd.AddCommand(newCmdSnapshotList(), newCmdSnapshotCreate(), newCmdSnapshotDelete())
	return cmd
}

func newCmdSnapshotList() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List snapshots", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()

		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}

		appID, err := resolveAppID(ctx, u.Repos.App, nil)
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

		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		snapshotName := flagVolumeSnapshotName
		source, _ := cmd.Flags().GetString("source")

		appID, err := resolveAppID(ctx, u.Repos.App, nil)
		if err != nil {
			return err
		}

		// Get app for logging
		app, err := u.Repos.App.Get(ctx, appID)
		if err != nil {
			return fmt.Errorf("failed to get app: %w", err)
		}
		logger.Info(ctx, "create snapshot", "app", app.Name, "volume", volName, "source", source, "name", snapshotName)
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

		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		snapName := flagVolumeSnapshotName
		if snapName == "" {
			return fmt.Errorf("--name (or --snap-name) required")
		}

		appID, err := resolveAppID(ctx, u.Repos.App, nil)
		if err != nil {
			return err
		}

		// Get app for logging
		app, err := u.Repos.App.Get(ctx, appID)
		if err != nil {
			return fmt.Errorf("failed to get app: %w", err)
		}
		logger.Info(ctx, "delete snapshot", "app", app.Name, "volume", volName, "snapshot", snapName)
		if _, err := u.SnapshotDelete(ctx, &vuc.SnapshotDeleteInput{AppID: appID, VolumeName: volName, SnapshotName: snapName}); err != nil {
			return err
		}
		return nil
	}}
	return cmd
}
