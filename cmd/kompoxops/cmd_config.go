package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/config"
)

// newCmdConfig returns a command that reads and shows the current configuration.
func newCmdConfig() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "config",
		Short: "Read and validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = config.DefaultConfigPath
			}
			cfg, err := config.OpsRead(file)
			if err != nil {
				return err
			}
			// Print a concise summary to stdout
			fmt.Fprintf(cmd.OutOrStdout(), "version=%d service=%s domain=%s cluster=%s provider=%s app=%s\n",
				cfg.Version, cfg.Service.Name, cfg.Service.Domain, cfg.Cluster.Name, cfg.Cluster.Provider, cfg.App.Name)
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", config.DefaultConfigPath, "Path to kompoxops.yml")
	return c
}
