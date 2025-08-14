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
	"github.com/spf13/pflag"
	"github.com/yaegashi/kompoxops/usecase/service"
	"gopkg.in/yaml.v3"
)

// serviceSpec is the YAML/JSON on-disk representation for create/update.
type serviceSpec struct {
	Name string `yaml:"name" json:"name"`
}

func newCmdAdminService() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "service",
		Short:         "Manage Service resources",
		RunE:          func(cmd *cobra.Command, args []string) error { return cmd.Help() },
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(newCmdAdminServiceList())
	cmd.AddCommand(newCmdAdminServiceGet())
	cmd.AddCommand(newCmdAdminServiceCreate())
	cmd.AddCommand(newCmdAdminServiceUpdate())
	cmd.AddCommand(newCmdAdminServiceDelete())
	return cmd
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

// buildServiceUseCase selects repository based on db-url flag.
func buildServiceUseCase(cmd *cobra.Command) (*service.UseCase, error) {
	serviceRepo, _, _, _, err := buildRepositories(cmd)
	if err != nil {
		return nil, err
	}
	return &service.UseCase{Services: serviceRepo}, nil
}

func newCmdAdminServiceList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List services",
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildServiceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			items, err := uc.List(ctx, service.ListServicesQuery{})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, it := range items {
				if err := enc.Encode(it); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newCmdAdminServiceGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildServiceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			s, err := uc.Get(ctx, args[0])
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(s)
		},
	}
}

func readServiceSpec(cmd *cobra.Command, path string) (*serviceSpec, error) {
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
	var spec serviceSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func newCmdAdminServiceCreate() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a service (from spec file)",
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildServiceUseCase(cmd)
			if err != nil {
				return err
			}
			spec, err := readServiceSpec(cmd, file)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := uc.Create(ctx, service.CreateServiceCommand{Name: spec.Name})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to service spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminServiceUpdate() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a service (merge from spec)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildServiceUseCase(cmd)
			if err != nil {
				return err
			}
			spec, err := readServiceSpec(cmd, file)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			var namePtr *string
			if spec.Name != "" {
				namePtr = &spec.Name
			}
			out, err := uc.Update(ctx, service.UpdateServiceCommand{ID: args[0], Name: namePtr})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to service spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminServiceDelete() *cobra.Command {
	c := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			uc, err := buildServiceUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			if err := uc.Delete(ctx, service.DeleteServiceCommand{ID: args[0]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	}
	return c
}
