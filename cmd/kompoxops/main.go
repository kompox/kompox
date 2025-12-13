package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	_ "github.com/kompox/kompox/adapters/drivers/provider/aks"
	_ "github.com/kompox/kompox/adapters/drivers/provider/k3s"
	"github.com/kompox/kompox/config/kompoxenv"
	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/naming"
	"github.com/spf13/cobra"
)

// Context keys
type contextKey string

const (
	kompoxEnvKey     contextKey = "kompox-env"
	logFilePathKey   contextKey = "log-file-path"
	logFileCloserKey contextKey = "log-file-closer"
)

// ExitCodeError is an error that carries an exit code.
// Use this to propagate subprocess exit codes to main() without calling os.Exit() directly.
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// getKompoxEnv retrieves the kompoxenv.Env from context.
func getKompoxEnv(ctx context.Context) *kompoxenv.Env {
	if env, ok := ctx.Value(kompoxEnvKey).(*kompoxenv.Env); ok {
		return env
	}
	return nil
}

// getLogFilePath retrieves the log file path from context.
func getLogFilePath(ctx context.Context) string {
	if path, ok := ctx.Value(logFilePathKey).(string); ok {
		return path
	}
	return ""
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

	cmd.PersistentFlags().String("log-format", "", "Log format (json|human) (env KOMPOX_LOG_FORMAT, default: json)")
	cmd.PersistentFlags().String("log-level", "", "Log level (debug|info|warn|error) (env KOMPOX_LOG_LEVEL, default: info)")
	cmd.PersistentFlags().String("log-output", "", "Log output: path, '-' for stderr, 'none' to disable (env KOMPOX_LOG_OUTPUT)")

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

		// Resolve logging configuration: flag > env > config.yml > default
		logCfg := resolveLogConfig(c, cfg)

		// Clean up old log files
		if err := logging.CleanupOldLogFiles(logCfg.Dir, logCfg.RetentionDays); err != nil {
			// Log cleanup failure is not fatal - continue
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old log files: %v\n", err)
		}

		// Create log file
		logFile, err := logging.NewLogFile(logCfg)
		if err != nil {
			return fmt.Errorf("initializing log file: %w", err)
		}

		// Parse log level
		var lvl slog.Level
		switch strings.ToLower(strings.TrimSpace(logCfg.Level)) {
		case "debug":
			lvl = slog.LevelDebug
		case "warn", "warning":
			lvl = slog.LevelWarn
		case "error", "err":
			lvl = slog.LevelError
		default:
			lvl = slog.LevelInfo
		}

		// Create logger with file output
		l, err := logging.NewWithWriter(logCfg.Format, lvl, logFile.Writer())
		if err != nil {
			logFile.Close()
			return err
		}

		// Generate runID and attach to logger
		runID, err := naming.NewCompactID()
		if err != nil {
			runID = "error"
		}
		l = l.With("runId", runID)

		// Emit CMD start log
		l.Info(ctx, "CMD", "args", os.Args)

		// Store log file path and closer in context
		ctx = context.WithValue(ctx, logFilePathKey, logFile.Path)
		ctx = context.WithValue(ctx, logFileCloserKey, logFile)
		ctx = logging.WithLogger(ctx, l)
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

// resolveLogConfig resolves logging configuration from multiple sources.
// Priority: flag > env > config.yml > default
func resolveLogConfig(c *cobra.Command, cfg *kompoxenv.Env) *logging.LogConfig {
	logCfg := &logging.LogConfig{
		Format:        "json",
		Level:         "INFO",
		Output:        "",
		Dir:           filepath.Join(cfg.KompoxDir, "logs"),
		RetentionDays: 7,
	}

	// Apply config.yml values
	if cfg.Logging.Dir != "" {
		logCfg.Dir = cfg.ExpandVars(cfg.Logging.Dir)
	}
	if cfg.Logging.Format != "" {
		logCfg.Format = cfg.Logging.Format
	}
	if cfg.Logging.Level != "" {
		logCfg.Level = cfg.Logging.Level
	}
	if cfg.Logging.RetentionDays > 0 {
		logCfg.RetentionDays = cfg.Logging.RetentionDays
	}

	// Apply environment variables
	if env := os.Getenv("KOMPOX_LOG_DIR"); env != "" {
		logCfg.Dir = env
	}
	if env := os.Getenv("KOMPOX_LOG_FORMAT"); env != "" {
		logCfg.Format = env
	}
	if env := os.Getenv("KOMPOX_LOG_LEVEL"); env != "" {
		logCfg.Level = env
	}
	if env := os.Getenv("KOMPOX_LOG_OUTPUT"); env != "" {
		logCfg.Output = env
	}

	// Apply flags (highest priority)
	if format, _ := c.Flags().GetString("log-format"); format != "" {
		logCfg.Format = format
	}
	if level, _ := c.Flags().GetString("log-level"); level != "" {
		logCfg.Level = level
	}
	if output, _ := c.Flags().GetString("log-output"); output != "" {
		logCfg.Output = output
	}

	return logCfg
}

func main() {
	root := newRootCmd()
	root.SetContext(context.Background())
	executed, err := root.ExecuteC()

	// Determine exit code
	exitCode := 0
	if err != nil {
		exitCode = 1
		// Check if error carries a specific exit code
		var exitErr ExitCodeError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.Code
			err = nil // Don't print error message for exit code propagation
		}
	}

	// Get context from executed command
	ctx := root.Context()
	if executed != nil {
		ctx = executed.Context()
	}

	// Emit CMD exit log
	logging.FromContext(ctx).Info(ctx, "CMD", "exitCode", exitCode)

	// Close log file if opened
	if closer, ok := ctx.Value(logFileCloserKey).(*logging.LogFile); ok && closer != nil {
		closer.Close()
	}

	if err != nil {
		// Print error message
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)

		// Print log file path if available
		if logPath := getLogFilePath(ctx); logPath != "" {
			fmt.Fprintf(os.Stderr, "See log file for details: %s\n", logPath)
		}
	}
	os.Exit(exitCode)
}
