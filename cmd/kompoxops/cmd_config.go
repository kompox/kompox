package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/kompox/kompox/config/kompoxopscfg"
)

// newCmdConfig returns a command that reads and shows the current configuration.
func newCmdConfig() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:                "config",
		Short:              "Read and validate configuration",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = kompoxopscfg.DefaultConfigPath
			}
			cfg, err := kompoxopscfg.Load(file)
			if err != nil {
				return err
			}
			// Determine effective domain from cluster.ingress.domain
			domain := strings.TrimSpace(cfg.Cluster.Ingress.Domain)
			// Print a concise summary to stdout
			fmt.Fprintf(cmd.OutOrStdout(), "version=%s service=%s provider=%s(%s) cluster=%s domain=%s app=%s\n",
				cfg.Version, cfg.Service.Name, cfg.Provider.Name, cfg.Provider.Driver, cfg.Cluster.Name, domain, cfg.App.Name)
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "f", kompoxopscfg.DefaultConfigPath, "Path to kompoxops.yml")
	return c
}
