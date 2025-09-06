package main

import (
	"fmt"
	"runtime"

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
			fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", commit)
			fmt.Fprintf(cmd.OutOrStdout(), "  built: %s\n", date)
			fmt.Fprintf(cmd.OutOrStdout(), "  go: %s\n", runtime.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
