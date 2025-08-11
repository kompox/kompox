package main

import (
	"github.com/spf13/cobra"
)

// newCmdCluster returns the parent command for cluster-related operations.
func newCmdCluster() *cobra.Command {
	c := &cobra.Command{
		Use:   "cluster",
		Short: "Cluster related commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Show help if no subcommand provided
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.AddCommand(newCmdClusterInfo())
	c.AddCommand(newCmdClusterPing())
	c.AddCommand(newCmdClusterDeploy())
	return c
}
