package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/usecase/cluster"
	"gopkg.in/yaml.v3"
)

type clusterSpec struct {
	Name       string `yaml:"name" json:"name"`
	ProviderID string `yaml:"providerID,omitempty" json:"providerID,omitempty"`
}

func newCmdAdminCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "cluster",
		Short:              "Manage Cluster resources",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}
	cmd.AddCommand(
		newCmdAdminClusterList(),
		newCmdAdminClusterGet(),
		newCmdAdminClusterCreate(),
		newCmdAdminClusterUpdate(),
		newCmdAdminClusterDelete(),
		newCmdAdminClusterProvision(),
		newCmdAdminClusterDeprovision(),
	)
	return cmd
}

func newCmdAdminClusterList() *cobra.Command {
	return &cobra.Command{
		Use:                "list",
		Short:              "List clusters",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := buildClusterUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := u.List(ctx, &cluster.ListInput{})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, it := range out.Clusters {
				if err := enc.Encode(it); err != nil {
					return err
				}
			}
			return nil
		}}
}

func newCmdAdminClusterGet() *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get a cluster", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		out, err := u.Get(ctx, &cluster.GetInput{ClusterID: args[0]})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Cluster)
	}}
}

func readClusterSpec(cmd *cobra.Command, path string) (*clusterSpec, error) {
	if path == "" {
		return nil, errors.New("spec file required (-f)")
	}
	var r io.Reader
	if path == "-" {
		r = cmd.InOrStdin()
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var spec clusterSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func newCmdAdminClusterCreate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "create", Short: "Create a cluster (from spec file)", RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readClusterSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		out, err := u.Create(ctx, &cluster.CreateInput{Name: spec.Name, ProviderID: spec.ProviderID})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Cluster)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to cluster spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminClusterUpdate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "update <id>", Short: "Update a cluster (merge from spec)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readClusterSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		var namePtr, providerPtr *string
		if spec.Name != "" {
			namePtr = &spec.Name
		}
		if spec.ProviderID != "" {
			providerPtr = &spec.ProviderID
		}
		out, err := u.Update(ctx, &cluster.UpdateInput{ClusterID: args[0], Name: namePtr, ProviderID: providerPtr})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.Cluster)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to cluster spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminClusterDelete() *cobra.Command {
	return &cobra.Command{Use: "delete <id>", Short: "Delete a cluster", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if _, err := u.Delete(ctx, &cluster.DeleteInput{ClusterID: args[0]}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
		return nil
	}}
}

func newCmdAdminClusterProvision() *cobra.Command {
	return &cobra.Command{Use: "provision <id>", Short: "Provision a cluster", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
		defer cancel()
		if _, err := u.Provision(ctx, &cluster.ProvisionInput{ClusterID: args[0]}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "provisioned %s\n", args[0])
		return nil
	}}
}

func newCmdAdminClusterDeprovision() *cobra.Command {
	return &cobra.Command{Use: "deprovision <id>", Short: "Deprovision a cluster", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildClusterUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Minute)
		defer cancel()
		if _, err := u.Deprovision(ctx, &cluster.DeprovisionInput{ClusterID: args[0]}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deprovisioned %s\n", args[0])
		return nil
	}}
}
