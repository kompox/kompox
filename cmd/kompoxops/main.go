package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"log/slog"

	_ "github.com/kompox/kompox/adapters/drivers/provider/aks"
	_ "github.com/kompox/kompox/adapters/drivers/provider/k3s"
	"github.com/kompox/kompox/internal/logging"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "kompoxops",
		Short:              "KompoxOps CLI",
		Long:               "KompoxOps CLI",
		Version:            version,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
	}

	// Add global db-url flag
	defaultDB := os.Getenv("KOMPOX_DB_URL")
	if defaultDB == "" {
		defaultDB = "file:kompoxops.yml"
	}
	cmd.PersistentFlags().String("db-url", defaultDB, "Database URL (env KOMPOX_DB_URL) (file:/path/to/kompoxops.yml | sqlite:/path/to.db | postgres:// | mysql://)")

	// Add KOM flags
	cmd.PersistentFlags().StringArray("kom-path", nil, "KOM YAML paths (files or directories, can be repeated) (env KOMPOX_KOM_PATH, comma-separated)")
	cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "KOM YAML for app inference (env KOMPOX_KOM_APP)")

	cmd.PersistentFlags().String("log-format", "human", "Log format (human|text|json) (env KOMPOX_LOG_FORMAT)")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug|info|warn|error) (env KOMPOX_LOG_LEVEL)")

	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		// Initialize KOM mode first (before logging setup). This also handles legacy env fail-fast.
		if err := initializeKOMMode(c); err != nil {
			return fmt.Errorf("KOM initialization failed: %w", err)
		}

		format, _ := c.Flags().GetString("log-format")
		if env := os.Getenv("KOMPOX_LOG_FORMAT"); env != "" { // env overrides flag
			format = env
		}
		levelStr, _ := c.Flags().GetString("log-level")
		if env := os.Getenv("KOMPOX_LOG_LEVEL"); env != "" { // env overrides flag
			levelStr = env
		}
		var lvl slog.Level
		switch strings.ToLower(strings.TrimSpace(levelStr)) {
		case "debug":
			lvl = slog.LevelDebug
		case "warn", "warning":
			lvl = slog.LevelWarn
		case "error", "err":
			lvl = slog.LevelError
		default:
			lvl = slog.LevelInfo
		}
		l, err := logging.New(format, lvl)
		if err != nil {
			return err
		}
		ctx := logging.WithLogger(c.Context(), l)
		c.SetContext(ctx)
		return nil
	}

	cmd.AddCommand(newCmdVersion())
	cmd.AddCommand(newCmdConfig())
	cmd.AddCommand(newCmdCluster())
	cmd.AddCommand(newCmdApp())
	cmd.AddCommand(newCmdSecret())
	cmd.AddCommand(newCmdBox())
	cmd.AddCommand(newCmdDisk())
	cmd.AddCommand(newCmdSnapshot())
	cmd.AddCommand(newCmdDNS())
	cmd.AddCommand(newCmdAdmin())
	return cmd
}

func main() {
	root := newRootCmd()
	root.SetContext(context.Background())
	executed, err := root.ExecuteC()
	if err != nil {
		ctx := root.Context()
		if executed != nil {
			ctx = executed.Context()
		}
		logging.FromContext(ctx).Errorf(ctx, "Failed: %s", err)
		os.Exit(1)
	}
}
