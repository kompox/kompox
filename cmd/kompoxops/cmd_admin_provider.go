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
	uc "github.com/yaegashi/kompoxops/usecase/provider"
	"gopkg.in/yaml.v3"
)

type providerSpec struct {
	Name   string `yaml:"name" json:"name"`
	Driver string `yaml:"driver" json:"driver"`
}

func newCmdAdminProvider() *cobra.Command {
	cmd := &cobra.Command{Use: "provider", Short: "Manage Provider resources", RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }, SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newCmdAdminProviderList(), newCmdAdminProviderGet(), newCmdAdminProviderCreate(), newCmdAdminProviderUpdate(), newCmdAdminProviderDelete())
	return cmd
}

func buildProviderUseCase(cmd *cobra.Command) (*uc.UseCase, error) {
	_, providerRepo, _, _, err := buildRepositories(cmd)
	if err != nil {
		return nil, err
	}
	return &uc.UseCase{Providers: providerRepo}, nil
}

func newCmdAdminProviderList() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List providers", RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildProviderUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		items, err := u.List(ctx)
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
	}}
}

func newCmdAdminProviderGet() *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get a provider", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildProviderUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		p, err := u.Get(ctx, args[0])
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(p)
	}}
}

func readProviderSpec(cmd *cobra.Command, path string) (*providerSpec, error) {
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
	var spec providerSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func newCmdAdminProviderCreate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "create", Short: "Create a provider (from spec file)", RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildProviderUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readProviderSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		out, err := u.Create(ctx, uc.CreateInput{Name: spec.Name, Driver: spec.Driver})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to provider spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminProviderUpdate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "update <id>", Short: "Update a provider (merge from spec)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildProviderUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readProviderSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		var namePtr, driverPtr *string
		if spec.Name != "" {
			namePtr = &spec.Name
		}
		if spec.Driver != "" {
			driverPtr = &spec.Driver
		}
		out, err := u.Update(ctx, uc.UpdateInput{ID: args[0], Name: namePtr, Driver: driverPtr})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to provider spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminProviderDelete() *cobra.Command {
	return &cobra.Command{Use: "delete <id>", Short: "Delete a provider", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildProviderUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if err := u.Delete(ctx, uc.DeleteInput{ID: args[0]}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
		return nil
	}}
}
