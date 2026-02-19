package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
	nuc "github.com/kompox/kompox/usecase/nodepool"
	"github.com/spf13/cobra"
)

type nodePoolPortMock struct {
	listFn   func(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error)
	createFn func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error)
	updateFn func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error)
	deleteFn func(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error
}

type clusterRepoMock struct {
	getFn func(ctx context.Context, id string) (*model.Cluster, error)
}

func (m *clusterRepoMock) Create(ctx context.Context, c *model.Cluster) error {
	return errors.New("not implemented")
}
func (m *clusterRepoMock) List(ctx context.Context) ([]*model.Cluster, error) {
	return nil, errors.New("not implemented")
}
func (m *clusterRepoMock) Update(ctx context.Context, c *model.Cluster) error {
	return errors.New("not implemented")
}
func (m *clusterRepoMock) Delete(ctx context.Context, id string) error {
	return errors.New("not implemented")
}
func (m *clusterRepoMock) Get(ctx context.Context, id string) (*model.Cluster, error) {
	if m.getFn == nil {
		return nil, errors.New("not implemented")
	}
	return m.getFn(ctx, id)
}

func (m *nodePoolPortMock) NodePoolList(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error) {
	if m.listFn == nil {
		return nil, errors.New("not implemented")
	}
	return m.listFn(ctx, cluster, opts...)
}

func (m *nodePoolPortMock) NodePoolCreate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error) {
	if m.createFn == nil {
		return nil, errors.New("not implemented")
	}
	return m.createFn(ctx, cluster, pool, opts...)
}

func (m *nodePoolPortMock) NodePoolUpdate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error) {
	if m.updateFn == nil {
		return nil, errors.New("not implemented")
	}
	return m.updateFn(ctx, cluster, pool, opts...)
}

func (m *nodePoolPortMock) NodePoolDelete(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error {
	if m.deleteFn == nil {
		return errors.New("not implemented")
	}
	return m.deleteFn(ctx, cluster, poolName, opts...)
}

func TestLoadNodePoolSpec(t *testing.T) {
	t.Run("yaml_success", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "pool.yaml")
		if err := os.WriteFile(path, []byte("name: pool1\nmode: user\n"), 0600); err != nil {
			t.Fatalf("write spec: %v", err)
		}
		spec, err := loadNodePoolSpec(path)
		if err != nil {
			t.Fatalf("load spec: %v", err)
		}
		if spec.Name != "pool1" {
			t.Fatalf("name = %q, want pool1", spec.Name)
		}
	})

	t.Run("read_error", func(t *testing.T) {
		_, err := loadNodePoolSpec("/nonexistent/pool.yaml")
		if err == nil || !strings.Contains(err.Error(), "failed to read file") {
			t.Fatalf("expected read error, got: %v", err)
		}
	})

	t.Run("parse_error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "pool.yaml")
		if err := os.WriteFile(path, []byte("name: ["), 0600); err != nil {
			t.Fatalf("write spec: %v", err)
		}
		_, err := loadNodePoolSpec(path)
		if err == nil || !strings.Contains(err.Error(), "failed to parse YAML/JSON") {
			t.Fatalf("expected parse error, got: %v", err)
		}
	})
}

func TestNodePoolCreateCommand(t *testing.T) {
	origBuild := buildNodePoolUseCaseFunc
	origResolve := resolveClusterIDFunc
	defer func() {
		buildNodePoolUseCaseFunc = origBuild
		resolveClusterIDFunc = origResolve
	}()

	resolveClusterIDFunc = func(ctx context.Context, _ domain.ClusterRepository, _ []string) (string, error) {
		return "ws/prv/cls", nil
	}

	t.Run("file_required", func(t *testing.T) {
		buildNodePoolUseCaseFunc = func(cmd *cobra.Command) (*nuc.UseCase, error) {
			return &nuc.UseCase{Repos: &nuc.Repos{}}, nil
		}
		cmd := newCmdClusterNodePoolCreate()
		cmd.SetContext(context.Background())
		err := cmd.RunE(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--file is required") {
			t.Fatalf("expected --file error, got: %v", err)
		}
	})

	t.Run("maps_input_to_usecase", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "pool.yaml")
		content := "name: pool1\nmode: user\nlabels:\n  team: dev\nautoscaling:\n  enabled: true\n  min: 1\n  max: 2\n"
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("write spec: %v", err)
		}

		var gotPool model.NodePool
		var gotForce bool
		port := &nodePoolPortMock{
			createFn: func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error) {
				gotPool = pool
				gotForce = model.ApplyNodePoolCreateOptions(opts...).Force
				return &pool, nil
			},
		}

		buildNodePoolUseCaseFunc = func(cmd *cobra.Command) (*nuc.UseCase, error) {
			return &nuc.UseCase{Repos: &nuc.Repos{Cluster: &clusterRepoMock{getFn: func(ctx context.Context, id string) (*model.Cluster, error) {
				return &model.Cluster{ID: id}, nil
			}}}, NodePoolPort: port}, nil
		}

		cmd := newCmdClusterNodePoolCreate()
		cmd.SetContext(context.Background())
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		if err := cmd.Flags().Set("file", path); err != nil {
			t.Fatalf("set file flag: %v", err)
		}
		if err := cmd.Flags().Set("force", "true"); err != nil {
			t.Fatalf("set force flag: %v", err)
		}
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("run create: %v", err)
		}

		if gotPool.Name == nil || *gotPool.Name != "pool1" {
			t.Fatalf("pool name not mapped: %+v", gotPool)
		}
		if gotPool.Mode == nil || *gotPool.Mode != "user" {
			t.Fatalf("pool mode not mapped: %+v", gotPool)
		}
		if !gotForce {
			t.Fatalf("expected force option true")
		}
	})
}

func TestNodePoolUpdateAndDeleteValidation(t *testing.T) {
	origBuild := buildNodePoolUseCaseFunc
	origResolve := resolveClusterIDFunc
	defer func() {
		buildNodePoolUseCaseFunc = origBuild
		resolveClusterIDFunc = origResolve
	}()

	resolveClusterIDFunc = func(ctx context.Context, _ domain.ClusterRepository, _ []string) (string, error) {
		return "ws/prv/cls", nil
	}
	buildNodePoolUseCaseFunc = func(cmd *cobra.Command) (*nuc.UseCase, error) {
		return &nuc.UseCase{Repos: &nuc.Repos{}}, nil
	}

	t.Run("update_file_required", func(t *testing.T) {
		cmd := newCmdClusterNodePoolUpdate()
		cmd.SetContext(context.Background())
		err := cmd.RunE(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--file is required") {
			t.Fatalf("expected --file error, got: %v", err)
		}
	})

	t.Run("delete_name_required", func(t *testing.T) {
		cmd := newCmdClusterNodePoolDelete()
		cmd.SetContext(context.Background())
		err := cmd.RunE(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--name is required") {
			t.Fatalf("expected --name error, got: %v", err)
		}
	})
}

func TestNodePoolListCommandMapsNameFilter(t *testing.T) {
	origBuild := buildNodePoolUseCaseFunc
	origResolve := resolveClusterIDFunc
	defer func() {
		buildNodePoolUseCaseFunc = origBuild
		resolveClusterIDFunc = origResolve
	}()

	resolveClusterIDFunc = func(ctx context.Context, _ domain.ClusterRepository, _ []string) (string, error) {
		return "ws/prv/cls", nil
	}

	var gotNameFilter string
	port := &nodePoolPortMock{
		listFn: func(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error) {
			gotNameFilter = model.ApplyNodePoolListOptions(opts...).Name
			name := "pool1"
			return []*model.NodePool{{Name: &name}}, nil
		},
	}

	buildNodePoolUseCaseFunc = func(cmd *cobra.Command) (*nuc.UseCase, error) {
		return &nuc.UseCase{Repos: &nuc.Repos{Cluster: &clusterRepoMock{getFn: func(ctx context.Context, id string) (*model.Cluster, error) {
			return &model.Cluster{ID: id}, nil
		}}}, NodePoolPort: port}, nil
	}

	cmd := newCmdClusterNodePoolList()
	cmd.SetContext(context.Background())
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Flags().Set("name", "pool1"); err != nil {
		t.Fatalf("set name flag: %v", err)
	}

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("run list: %v", err)
	}
	if gotNameFilter != "pool1" {
		t.Fatalf("name filter not mapped: %q", gotNameFilter)
	}
}
