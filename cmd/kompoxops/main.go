package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"log/slog"

	_ "github.com/kompox/kompox/adapters/drivers/provider/aks"
	_ "github.com/kompox/kompox/adapters/drivers/provider/k3s"
	"github.com/kompox/kompox/config/kompoxenv"
	"github.com/kompox/kompox/internal/logging"
	"github.com/spf13/cobra"
)

// Context keys
type contextKey string

const (
	kompoxEnvKey contextKey = "kompox-env"
)

// getKompoxEnv retrieves the kompoxenv.Env from context.
func getKompoxEnv(ctx context.Context) *kompoxenv.Env {
	if env, ok := ctx.Value(kompoxEnvKey).(*kompoxenv.Env); ok {
		return env
	}
	return nil
}

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

	// Add directory flags
	cmd.PersistentFlags().StringP("chdir", "C", "", "Change to directory before processing")
	cmd.PersistentFlags().String("kompox-root", "", "Project directory (env KOMPOX_ROOT)")
	cmd.PersistentFlags().String("kompox-dir", "", "Kompox directory (env KOMPOX_DIR)")

	// Add KOM flags
	cmd.PersistentFlags().StringArray("kom-path", nil, "KOM YAML paths (files or directories, can be repeated) (env KOMPOX_KOM_PATH, comma-separated)")
	cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "KOM YAML for app inference (env KOMPOX_KOM_APP)")

	cmd.PersistentFlags().String("log-format", "human", "Log format (human|text|json) (env KOMPOX_LOG_FORMAT)")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug|info|warn|error) (env KOMPOX_LOG_LEVEL)")

	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		// Skip init command as it doesn't require kompoxenv
		if c.Name() == "init" {
			return nil
		}
		if dir, _ := c.Flags().GetString("chdir"); dir != "" {
			if err := os.Chdir(dir); err != nil {
				return fmt.Errorf("failed to change directory: %w", err)
			}
		} // Resolve KOMPOX_ROOT and KOMPOX_DIR
		kompoxRootFlag, _ := c.Flags().GetString("kompox-root")
		kompoxDirFlag, _ := c.Flags().GetString("kompox-dir")

		// Priority: flag > env > discovery/default
		kompoxRootVal := kompoxRootFlag
		if kompoxRootVal == "" {
			kompoxRootVal = os.Getenv(kompoxenv.KompoxRootEnvKey)
		}

		kompoxDirVal := kompoxDirFlag
		if kompoxDirVal == "" {
			kompoxDirVal = os.Getenv(kompoxenv.KompoxDirEnvKey)
		}

		// Get current working directory for discovery
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		// Resolve directories and load config
		cfg, err := kompoxenv.Resolve(kompoxRootVal, kompoxDirVal, workDir)
		if err != nil {
			return fmt.Errorf("resolving KOMPOX_ROOT/KOMPOX_DIR: %w", err)
		}

		// Export to environment
		if err := os.Setenv(kompoxenv.KompoxRootEnvKey, cfg.KompoxRoot); err != nil {
			return fmt.Errorf("setting KOMPOX_ROOT environment variable: %w", err)
		}
		if err := os.Setenv(kompoxenv.KompoxDirEnvKey, cfg.KompoxDir); err != nil {
			return fmt.Errorf("setting KOMPOX_DIR environment variable: %w", err)
		}

		// Store Config in context for use by commands
		ctx := context.WithValue(c.Context(), kompoxEnvKey, cfg)
		c.SetContext(ctx)

		// Initialize KOM mode (before logging setup). This also handles legacy env fail-fast.
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
		ctx = logging.WithLogger(c.Context(), l)
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
	cmd.AddCommand(newCmdInit())
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
