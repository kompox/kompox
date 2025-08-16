package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCmdAdmin returns the parent command for admin operations.
func newCmdAdmin() *cobra.Command {
	c := &cobra.Command{
		Use:                "admin",
		Short:              "Administrative commands (direct CRUD without auth)",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}
	c.AddCommand(newCmdAdminService())
	c.AddCommand(newCmdAdminProvider())
	c.AddCommand(newCmdAdminCluster())
	c.AddCommand(newCmdAdminApp())
	return c
}
