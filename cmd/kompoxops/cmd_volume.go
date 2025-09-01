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

func newCmdVolume() *cobra.Command {
	cmd := &cobra.Command{Use: "volume", Short: "Manage app volumes", SilenceUsage: true, SilenceErrors: true, DisableSuggestions: true, RunE: func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") }}
	cmd.PersistentFlags().StringVarP(&flagVolumeAppName, "app-name", "A", "", "App name (default: app.name in kompoxops.yml)")
	cmd.PersistentFlags().StringP("vol-name", "V", "", "Volume name (required for list/create/assign/delete)")
	cmd.PersistentFlags().StringP("vol-inst-name", "I", "", "Volume instance name (required for assign/delete)")
	cmd.AddCommand(newCmdVolumeList(), newCmdVolumeCreate(), newCmdVolumeAssign(), newCmdVolumeDelete())
	return cmd
}

func getVolumeAppName(_ *cobra.Command) (string, error) {
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

func newCmdVolumeList() *cobra.Command {
	cmd := &cobra.Command{Use: "list", Short: "List volume instances", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		appName, err := getVolumeAppName(cmd)
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
		out, err := u.InstanceList(ctx, &vuc.InstanceListInput{AppID: appID, VolumeName: volName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Items)
	}}
	return cmd
}

func newCmdVolumeCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getVolumeAppName(cmd)
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
		out, err := u.InstanceCreate(ctx, &vuc.InstanceCreateInput{AppID: appID, VolumeName: volName})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Instance)
	}}
	return cmd
}

func newCmdVolumeAssign() *cobra.Command {
	cmd := &cobra.Command{Use: "assign", Short: "Assign volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getVolumeAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		instName, _ := cmd.Flags().GetString("vol-inst-name")
		if instName == "" {
			return fmt.Errorf("--vol-inst-name required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "assign volume instance", "app", appName, "volume", volName, "instance", instName)
		if _, err := u.InstanceAssign(ctx, &vuc.InstanceAssignInput{AppID: appID, VolumeName: volName, VolumeInstanceName: instName}); err != nil {
			return err
		}
		return nil
	}}
	return cmd
}

func newCmdVolumeDelete() *cobra.Command {
	cmd := &cobra.Command{Use: "delete", Short: "Delete volume instance", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		u, err := buildVolumeUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		logger := logging.FromContext(ctx)
		appName, err := getVolumeAppName(cmd)
		if err != nil {
			return err
		}
		volName, _ := cmd.Flags().GetString("vol-name")
		if volName == "" {
			return fmt.Errorf("--vol-name required")
		}
		instName, _ := cmd.Flags().GetString("vol-inst-name")
		if instName == "" {
			return fmt.Errorf("--vol-inst-name required")
		}
		appID, err := resolveAppIDByName(ctx, u, appName)
		if err != nil {
			return err
		}
		logger.Info(ctx, "delete volume instance", "app", appName, "volume", volName, "instance", instName)
		if _, err := u.InstanceDelete(ctx, &vuc.InstanceDeleteInput{AppID: appID, VolumeName: volName, VolumeInstanceName: instName}); err != nil {
			return err
		}
		return nil
	}}
	return cmd
}
