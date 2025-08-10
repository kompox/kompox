package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the application version shown by --version.
// Updated during releases via -ldflags if needed.
var version = "0.1.0"

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
	// Add subcommands
	cmd.AddCommand(newVersionCmd())
	return cmd
}

// newVersionCmd returns a command that prints the application version.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			// Keep output minimal and script-friendly
			fmt.Fprintf(cmd.OutOrStdout(), "kompoxops version %s\n", version)
		},
	}
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
