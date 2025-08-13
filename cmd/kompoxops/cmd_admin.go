package main

import (
	"os"

	"github.com/spf13/cobra"
)

// newCmdAdmin returns the parent command for admin operations.
func newCmdAdmin() *cobra.Command {
	c := &cobra.Command{
		Use:   "admin",
		Short: "Administrative commands (direct CRUD without auth)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	defaultDB := os.Getenv("KOMPOX_DB_URL")
	if defaultDB == "" {
		defaultDB = "memory:"
	}
	c.PersistentFlags().String("db-url", defaultDB, "Database URL (env KOMPOX_DB_URL) (memory: | sqlite:/path/to.db | sqlite::memory: | postgres:// | mysql://)")
	c.AddCommand(newCmdAdminService())
	c.AddCommand(newCmdAdminProvider())
	c.AddCommand(newCmdAdminCluster())
	c.AddCommand(newCmdAdminApp())
	return c
}
