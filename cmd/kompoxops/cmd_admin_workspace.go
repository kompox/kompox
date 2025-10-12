package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kompox/kompox/usecase/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// workspaceSpec is the YAML/JSON on-disk representation for create/update.
type workspaceSpec struct {
	Name string `yaml:"name" json:"name"`
}

func newCmdAdminWorkspace() *cobra.Command {
	c := &cobra.Command{
		Use:                "workspace",
		Aliases:            []string{"ws"},
		Short:              "Workspace admin commands",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	c.AddCommand(newCmdAdminWorkspaceList())
	c.AddCommand(newCmdAdminWorkspaceGet())
	c.AddCommand(newCmdAdminWorkspaceCreate())
	c.AddCommand(newCmdAdminWorkspaceUpdate())
	c.AddCommand(newCmdAdminWorkspaceDelete())
	return c
}

// findFlag recursively searches parents for a flag.
func findFlag(cmd *cobra.Command, name string) *pflag.Flag {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup(name); f != nil {
			return f
		}
		if f := c.PersistentFlags().Lookup(name); f != nil {
			return f
		}
	}
	return nil
}

func newCmdAdminWorkspaceList() *cobra.Command {
	return &cobra.Command{
		Use:                "list",
		Short:              "List workspaces",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildWorkspaceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := uc.List(ctx, &workspace.ListInput{})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, it := range out.Workspaces {
				if err := enc.Encode(it); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newCmdAdminWorkspaceGet() *cobra.Command {
	return &cobra.Command{
		Use:                "get <id>",
		Short:              "Get a workspace",
		Args:               cobra.ExactArgs(1),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildWorkspaceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := uc.Get(ctx, &workspace.GetInput{WorkspaceID: args[0]})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Workspace)
		},
	}
}

func readWorkspaceSpec(cmd *cobra.Command, path string) (*workspaceSpec, error) {
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
	var spec workspaceSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func newCmdAdminWorkspaceCreate() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:                "create",
		Short:              "Create a workspace (from spec file)",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildWorkspaceUseCase(cmd)
			if err != nil {
				return err
			}
			spec, err := readWorkspaceSpec(cmd, file)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := uc.Create(ctx, &workspace.CreateInput{Name: spec.Name})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Workspace)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to workspace spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminWorkspaceUpdate() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:                "update <id>",
		Short:              "Update a workspace (merge from spec)",
		Args:               cobra.ExactArgs(1),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildWorkspaceUseCase(cmd)
			if err != nil {
				return err
			}
			spec, err := readWorkspaceSpec(cmd, file)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			var namePtr *string
			if spec.Name != "" {
				namePtr = &spec.Name
			}
			out, err := uc.Update(ctx, &workspace.UpdateInput{WorkspaceID: args[0], Name: namePtr})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out.Workspace)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to workspace spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminWorkspaceDelete() *cobra.Command {
	c := &cobra.Command{
		Use:                "delete <id>",
		Short:              "Delete a workspace",
		Args:               cobra.ExactArgs(1),
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildWorkspaceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			if _, err := uc.Delete(ctx, &workspace.DeleteInput{WorkspaceID: args[0]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	}
	return c
}
