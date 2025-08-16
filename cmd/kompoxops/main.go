package main

import (
	"context"
	"os"

	"log/slog"

	"github.com/spf13/cobra"
	_ "github.com/yaegashi/kompoxops/adapters/drivers/provider/aks"
	_ "github.com/yaegashi/kompoxops/adapters/drivers/provider/k3s"
	"github.com/yaegashi/kompoxops/internal/logging"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kompoxops",
		Short:   "KompoxOps CLI",
		Long:    "KompoxOps CLI",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help by default when no subcommand is provided.
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Add global db-url flag
	defaultDB := os.Getenv("KOMPOX_DB_URL")
	if defaultDB == "" {
		defaultDB = "file:kompoxops.yml"
	}
	cmd.PersistentFlags().String("db-url", defaultDB, "Database URL (env KOMPOX_DB_URL) (file:/path/to/kompoxops.yml | sqlite:/path/to.db | postgres:// | mysql://)")

	// global flags (db-url already exists)
	cmd.PersistentFlags().String("log-format", "human", "Log format (human|text|json) (env KOMPOX_LOG_FORMAT)")

	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		format, _ := c.Flags().GetString("log-format")
		if env := os.Getenv("KOMPOX_LOG_FORMAT"); env != "" { // env overrides flag
			format = env
		}
		l, err := logging.New(format, slog.LevelInfo)
		if err != nil {
			return err
		}
		ctx := logging.WithLogger(c.Context(), l)
		c.SetContext(ctx)
		return nil
	}

	// Add subcommands
	cmd.AddCommand(newCmdVersion())
	cmd.AddCommand(newCmdConfig())
	cmd.AddCommand(newCmdCluster())
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
