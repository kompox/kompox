package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// newCmdClusterInfo shows the cluster information from kompoxops.yml
func newCmdClusterInfo() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show cluster info",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = cfgops.DefaultConfigPath
			}
			cfg, err := cfgops.Load(file)
			if err != nil {
				return err
			}
			cl := cfg.Cluster
			// Keep output concise and script-friendly (key=value pairs)
			// Include primary fields and common nested ones
			fmt.Fprintf(cmd.OutOrStdout(), "name=%s provider=%s ingress.controller=%s ingress.namespace=%s auth.type=%s auth.context=%s kubeconfig=%s\n",
				cl.Name, cl.Provider, cl.Ingress.Controller, cl.Ingress.Namespace, cl.Auth.Type, cl.Auth.Context, cl.Auth.Kubeconfig)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", cfgops.DefaultConfigPath, "Path to kompoxops.yml")
	return cmd
}
