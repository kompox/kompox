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
	"github.com/kompox/kompox/usecase/app"
	"gopkg.in/yaml.v3"
)

type appSpec struct {
	Name      string `yaml:"name" json:"name"`
	ClusterID string `yaml:"clusterID,omitempty" json:"clusterID,omitempty"`
}

func newCmdAdminApp() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "app",
		Short:              "Manage App resources",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("invalid command")
		},
	}
	cmd.AddCommand(
		newCmdAdminAppList(),
		newCmdAdminAppGet(),
		newCmdAdminAppCreate(),
		newCmdAdminAppUpdate(),
		newCmdAdminAppDelete(),
	)
	return cmd
}

func newCmdAdminAppList() *cobra.Command {
	return &cobra.Command{
		Use:                "list",
		Short:              "List apps",
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableSuggestions: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := buildAppUseCase(cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			out, err := u.List(ctx, &app.ListInput{})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, it := range out.Apps {
				if err := enc.Encode(it); err != nil {
					return err
				}
			}
			return nil
		}}
}

func newCmdAdminAppGet() *cobra.Command {
	return &cobra.Command{Use: "get <id>", Short: "Get an app", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildAppUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		out, err := u.Get(ctx, &app.GetInput{AppID: args[0]})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.App)
	}}
}

func readAppSpec(cmd *cobra.Command, path string) (*appSpec, error) {
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
	var spec appSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func newCmdAdminAppCreate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "create", Short: "Create an app (from spec file)", RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildAppUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readAppSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		out, err := u.Create(ctx, &app.CreateInput{Name: spec.Name, ClusterID: spec.ClusterID})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.App)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to app spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminAppUpdate() *cobra.Command {
	var file string
	c := &cobra.Command{Use: "update <id>", Short: "Update an app (merge from spec)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildAppUseCase(cmd)
		if err != nil {
			return err
		}
		spec, err := readAppSpec(cmd, file)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		var namePtr, clusterPtr *string
		if spec.Name != "" {
			namePtr = &spec.Name
		}
		if spec.ClusterID != "" {
			clusterPtr = &spec.ClusterID
		}
		out, err := u.Update(ctx, &app.UpdateInput{AppID: args[0], Name: namePtr, ClusterID: clusterPtr})
		if err != nil {
			return err
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out.App)
	}}
	c.Flags().StringVarP(&file, "file", "f", "", "Path to app spec (YAML), or '-' for stdin")
	_ = c.MarkFlagRequired("file")
	return c
}

func newCmdAdminAppDelete() *cobra.Command {
	return &cobra.Command{Use: "delete <id>", Short: "Delete an app", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		u, err := buildAppUseCase(cmd)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if _, err := u.Delete(ctx, &app.DeleteInput{AppID: args[0]}); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
		return nil
	}}
}
