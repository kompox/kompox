package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// newCmdConfig returns a command that reads and shows the current configuration.
func newCmdConfig() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "config",
		Short: "Read and validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = cfgops.DefaultConfigPath
			}
			cfg, err := cfgops.Load(file)
			if err != nil {
				return err
			}
			// Print a concise summary to stdout
			fmt.Fprintf(cmd.OutOrStdout(), "version=%s service=%s provider=%s(%s) cluster=%s domain=%s app=%s\n",
				cfg.Version, cfg.Service.Name, cfg.Provider.Name, cfg.Provider.Driver, cfg.Cluster.Name, cfg.Cluster.Domain, cfg.App.Name)
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", cfgops.DefaultConfigPath, "Path to kompoxops.yml")
	return c
}
