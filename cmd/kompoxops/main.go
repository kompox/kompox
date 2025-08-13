package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	_ "github.com/yaegashi/kompoxops/adapters/drivers/provider/aks"
	_ "github.com/yaegashi/kompoxops/adapters/drivers/provider/k3s"
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

	// Add subcommands
	cmd.AddCommand(newCmdVersion())
	cmd.AddCommand(newCmdConfig())
	cmd.AddCommand(newCmdCluster())
	cmd.AddCommand(newCmdAdmin())
	return cmd
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
