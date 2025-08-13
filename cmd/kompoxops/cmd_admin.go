package main

import "github.com/spf13/cobra"

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
	c.PersistentFlags().String("db-url", "memory:", "Database URL (memory: | sqlite:... | postgres:// | mysql://)")
	c.AddCommand(newCmdAdminService())
	return c
}
