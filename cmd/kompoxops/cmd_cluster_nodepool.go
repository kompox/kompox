package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/kompox/kompox/domain/model"
	nuc "github.com/kompox/kompox/usecase/nodepool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newCmdClusterNodePool returns the root nodepool command.
func newCmdClusterNodePool() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "nodepool",
		Short:              "Manage cluster node pools",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}
	cmd.AddCommand(
		newCmdClusterNodePoolList(),
		newCmdClusterNodePoolCreate(),
		newCmdClusterNodePoolUpdate(),
		newCmdClusterNodePoolDelete(),
	)
	return cmd
}

// newCmdClusterNodePoolList lists node pools in a cluster.
func newCmdClusterNodePoolList() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:                "list",
		Short:              "List node pools",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			u, err := buildNodePoolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()

			clusterID, err := resolveClusterID(ctx, u.Repos.Cluster, nil)
			if err != nil {
				return err
			}

			out, err := u.List(ctx, &nuc.ListInput{
				ClusterID: clusterID,
				Name:      name,
			})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Items)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Filter by node pool name (optional)")
	return cmd
}

// nodePoolSpec represents the YAML/JSON input schema for create/update operations.
type nodePoolSpec struct {
	Name          string             `yaml:"name" json:"name"`
	ProviderName  string             `yaml:"providerName,omitempty" json:"providerName,omitempty"`
	Mode          string             `yaml:"mode,omitempty" json:"mode,omitempty"`
	Labels        *map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Zones         *[]string          `yaml:"zones,omitempty" json:"zones,omitempty"`
	InstanceType  string             `yaml:"instanceType,omitempty" json:"instanceType,omitempty"`
	OSDiskType    string             `yaml:"osDiskType,omitempty" json:"osDiskType,omitempty"`
	OSDiskSizeGiB *int               `yaml:"osDiskSizeGiB,omitempty" json:"osDiskSizeGiB,omitempty"`
	Priority      string             `yaml:"priority,omitempty" json:"priority,omitempty"`
	Autoscaling   *struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
		Min     *int `yaml:"min,omitempty" json:"min,omitempty"`
		Max     *int `yaml:"max,omitempty" json:"max,omitempty"`
		Desired *int `yaml:"desired,omitempty" json:"desired,omitempty"`
	} `yaml:"autoscaling,omitempty" json:"autoscaling,omitempty"`
	Extensions map[string]any `yaml:"extensions,omitempty" json:"extensions,omitempty"`
}

// toModelNodePool converts a nodePoolSpec to a model.NodePool.
func (s *nodePoolSpec) toModelNodePool() model.NodePool {
	pool := model.NodePool{
		Extensions: s.Extensions,
	}
	if s.Name != "" {
		pool.Name = &s.Name
	}
	if s.ProviderName != "" {
		pool.ProviderName = &s.ProviderName
	}
	if s.Mode != "" {
		pool.Mode = &s.Mode
	}
	if s.Labels != nil {
		pool.Labels = s.Labels
	}
	if s.Zones != nil {
		pool.Zones = s.Zones
	}
	if s.InstanceType != "" {
		pool.InstanceType = &s.InstanceType
	}
	if s.OSDiskType != "" {
		pool.OSDiskType = &s.OSDiskType
	}
	if s.OSDiskSizeGiB != nil {
		pool.OSDiskSizeGiB = s.OSDiskSizeGiB
	}
	if s.Priority != "" {
		pool.Priority = &s.Priority
	}
	if s.Autoscaling != nil {
		as := &model.NodePoolAutoscaling{
			Enabled: s.Autoscaling.Enabled,
			Desired: s.Autoscaling.Desired,
		}
		if s.Autoscaling.Min != nil {
			as.Min = *s.Autoscaling.Min
		}
		if s.Autoscaling.Max != nil {
			as.Max = *s.Autoscaling.Max
		}
		pool.Autoscaling = as
	}
	return pool
}

// loadNodePoolSpec loads a node pool spec from a YAML or JSON file.
func loadNodePoolSpec(path string) (*nodePoolSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var spec nodePoolSpec
	// Try YAML first (YAML parser can handle JSON as well)
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML/JSON: %w", err)
	}

	return &spec, nil
}

// newCmdClusterNodePoolCreate creates a new node pool.
func newCmdClusterNodePoolCreate() *cobra.Command {
	var file string
	var force bool
	cmd := &cobra.Command{
		Use:                "create",
		Short:              "Create a node pool",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			u, err := buildNodePoolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			clusterID, err := resolveClusterID(ctx, u.Repos.Cluster, nil)
			if err != nil {
				return err
			}

			if file == "" {
				return fmt.Errorf("--file is required")
			}

			spec, err := loadNodePoolSpec(file)
			if err != nil {
				return err
			}

			if spec.Name == "" {
				return fmt.Errorf("name is required in the specification file")
			}

			pool := spec.toModelNodePool()

			resourceID := clusterID + "/nodepool:" + spec.Name
			ctx, cleanup := withCmdRunLogger(ctx, "nodepool.create", resourceID)
			defer func() { cleanup(err) }()

			out, err := u.Create(ctx, &nuc.CreateInput{
				ClusterID: clusterID,
				Pool:      pool,
				Force:     force,
			})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Pool)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to YAML or JSON file containing node pool specification (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Force creation if driver supports it")
	return cmd
}

// newCmdClusterNodePoolUpdate updates an existing node pool.
func newCmdClusterNodePoolUpdate() *cobra.Command {
	var file string
	var force bool
	cmd := &cobra.Command{
		Use:                "update",
		Short:              "Update a node pool",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			u, err := buildNodePoolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			clusterID, err := resolveClusterID(ctx, u.Repos.Cluster, nil)
			if err != nil {
				return err
			}

			if file == "" {
				return fmt.Errorf("--file is required")
			}

			spec, err := loadNodePoolSpec(file)
			if err != nil {
				return err
			}

			if spec.Name == "" {
				return fmt.Errorf("name is required in the specification file")
			}

			pool := spec.toModelNodePool()

			resourceID := clusterID + "/nodepool:" + spec.Name
			ctx, cleanup := withCmdRunLogger(ctx, "nodepool.update", resourceID)
			defer func() { cleanup(err) }()

			out, err := u.Update(ctx, &nuc.UpdateInput{
				ClusterID: clusterID,
				Pool:      pool,
				Force:     force,
			})
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Pool)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to YAML or JSON file containing node pool specification (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Force update if driver supports it")
	return cmd
}

// newCmdClusterNodePoolDelete deletes a node pool.
func newCmdClusterNodePoolDelete() *cobra.Command {
	var name string
	var force bool
	cmd := &cobra.Command{
		Use:                "delete",
		Short:              "Delete a node pool",
		Args:               cobra.NoArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			u, err := buildNodePoolUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			clusterID, err := resolveClusterID(ctx, u.Repos.Cluster, nil)
			if err != nil {
				return err
			}

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			resourceID := clusterID + "/nodepool:" + name
			ctx, cleanup := withCmdRunLogger(ctx, "nodepool.delete", resourceID)
			defer func() { cleanup(err) }()

			_, err = u.Delete(ctx, &nuc.DeleteInput{
				ClusterID: clusterID,
				Name:      name,
				Force:     force,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Node pool name (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion if driver supports it")
	return cmd
}
