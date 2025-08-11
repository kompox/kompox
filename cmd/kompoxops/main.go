package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	// Add subcommands
	cmd.AddCommand(newCmdVersion())
	cmd.AddCommand(newCmdConfig())
	cmd.AddCommand(newCmdCluster())
	return cmd
}

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
