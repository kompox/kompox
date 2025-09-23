package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCmdSecret is a new root-level group for managing secrets (env overrides, image pull auth, etc.).
func newCmdSecret() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "secret",
		Short:              "Manage app-related secrets (env overrides, registry auth)",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE:               func(cmd *cobra.Command, args []string) error { return fmt.Errorf("invalid command") },
	}
	// Attach subcommands.
	cmd.AddCommand(newCmdSecretEnv())
	cmd.AddCommand(newCmdSecretPull())
	return cmd
}
