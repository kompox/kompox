package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCmdVersion returns a command that prints the application version.
func newCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			// Keep output minimal and script-friendly
			fmt.Fprintf(cmd.OutOrStdout(), "kompoxops version %s\n", version)
		},
	}
}
