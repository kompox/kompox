package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCmdVersion returns a command that prints the application version.
func newCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:                "version",
		Short:              "Print the version",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "kompoxops version %s\n", version)
		},
	}
}
