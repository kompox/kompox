package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/cluster"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// newCmdClusterPing connects to the cluster and prints the API server version as a liveness check.
func newCmdClusterPing() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping Kubernetes API server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				file = cfgops.DefaultConfigPath
			}
			cfg, err := cfgops.Load(file)
			if err != nil {
				return err
			}
			cl, err := cluster.New(cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			if err := cl.Ping(ctx); err != nil {
				return err
			}
			ver, err := cl.APIServerVersion(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ok version=%s context=%s kubeconfig=%s\n", ver, cl.Context, cl.Kubeconf)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", cfgops.DefaultConfigPath, "Path to kompoxops.yml")
	return cmd
}
