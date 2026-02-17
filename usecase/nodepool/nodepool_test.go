package nodepool

import (
	"context"
	"errors"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

// mockClusterRepo is a mock implementation for testing.
type mockClusterRepo struct {
	getFunc func(ctx context.Context, id string) (*model.Cluster, error)
}

func (m *mockClusterRepo) Get(ctx context.Context, id string) (*model.Cluster, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClusterRepo) List(ctx context.Context) ([]*model.Cluster, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClusterRepo) Create(ctx context.Context, cluster *model.Cluster) error {
	return errors.New("not implemented")
}

func (m *mockClusterRepo) Update(ctx context.Context, cluster *model.Cluster) error {
	return errors.New("not implemented")
}

func (m *mockClusterRepo) Delete(ctx context.Context, id string) error {
	return errors.New("not implemented")
}

// mockNodePoolPort is a mock implementation for testing.
type mockNodePoolPort struct {
	listFunc   func(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error)
	createFunc func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error)
	updateFunc func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error)
	deleteFunc func(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error
}

func (m *mockNodePoolPort) NodePoolList(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, cluster, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockNodePoolPort) NodePoolCreate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, cluster, pool, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockNodePoolPort) NodePoolUpdate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, cluster, pool, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockNodePoolPort) NodePoolDelete(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, cluster, poolName, opts...)
	}
	return errors.New("not implemented")
}

func TestList(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		clusterID := "ws1/prv1/cls1"
		cluster := &model.Cluster{ID: clusterID, Name: "test-cluster"}
		poolName := "pool1"
		expectedPools := []*model.NodePool{
			{Name: &poolName},
		}

		repos := &Repos{
			Cluster: &mockClusterRepo{
				getFunc: func(ctx context.Context, id string) (*model.Cluster, error) {
					if id == clusterID {
						return cluster, nil
					}
					return nil, errors.New("not found")
				},
			},
		}

		port := &mockNodePoolPort{
			listFunc: func(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error) {
				return expectedPools, nil
			},
		}

		uc := &UseCase{Repos: repos, NodePoolPort: port}
		out, err := uc.List(ctx, &ListInput{ClusterID: clusterID})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out.Items) != 1 {
			t.Errorf("expected 1 pool, got %d", len(out.Items))
		}
	})

	t.Run("nil input", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.List(ctx, nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})

	t.Run("empty cluster ID", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.List(ctx, &ListInput{})
		if err == nil {
			t.Error("expected error for empty cluster ID")
		}
	})

	t.Run("cluster not found", func(t *testing.T) {
		repos := &Repos{
			Cluster: &mockClusterRepo{
				getFunc: func(ctx context.Context, id string) (*model.Cluster, error) {
					return nil, errors.New("cluster not found")
				},
			},
		}
		uc := &UseCase{Repos: repos}
		_, err := uc.List(ctx, &ListInput{ClusterID: "nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent cluster")
		}
	})
}

func TestCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		clusterID := "ws1/prv1/cls1"
		cluster := &model.Cluster{ID: clusterID, Name: "test-cluster"}
		poolName := "pool1"
		pool := model.NodePool{Name: &poolName}

		repos := &Repos{
			Cluster: &mockClusterRepo{
				getFunc: func(ctx context.Context, id string) (*model.Cluster, error) {
					return cluster, nil
				},
			},
		}

		port := &mockNodePoolPort{
			createFunc: func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error) {
				return &pool, nil
			},
		}

		uc := &UseCase{Repos: repos, NodePoolPort: port}
		out, err := uc.Create(ctx, &CreateInput{ClusterID: clusterID, Pool: pool})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Pool.Name == nil || *out.Pool.Name != poolName {
			t.Errorf("expected pool name %s, got %v", poolName, out.Pool.Name)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Create(ctx, nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})

	t.Run("empty cluster ID", func(t *testing.T) {
		poolName := "pool1"
		uc := &UseCase{}
		_, err := uc.Create(ctx, &CreateInput{Pool: model.NodePool{Name: &poolName}})
		if err == nil {
			t.Error("expected error for empty cluster ID")
		}
	})

	t.Run("empty pool name", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Create(ctx, &CreateInput{ClusterID: "ws1/prv1/cls1", Pool: model.NodePool{}})
		if err == nil {
			t.Error("expected error for empty pool name")
		}
	})
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		clusterID := "ws1/prv1/cls1"
		cluster := &model.Cluster{ID: clusterID, Name: "test-cluster"}
		poolName := "pool1"
		pool := model.NodePool{Name: &poolName}

		repos := &Repos{
			Cluster: &mockClusterRepo{
				getFunc: func(ctx context.Context, id string) (*model.Cluster, error) {
					return cluster, nil
				},
			},
		}

		port := &mockNodePoolPort{
			updateFunc: func(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error) {
				return &pool, nil
			},
		}

		uc := &UseCase{Repos: repos, NodePoolPort: port}
		out, err := uc.Update(ctx, &UpdateInput{ClusterID: clusterID, Pool: pool})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Pool.Name == nil || *out.Pool.Name != poolName {
			t.Errorf("expected pool name %s, got %v", poolName, out.Pool.Name)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Update(ctx, nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})
}

func TestDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		clusterID := "ws1/prv1/cls1"
		cluster := &model.Cluster{ID: clusterID, Name: "test-cluster"}

		repos := &Repos{
			Cluster: &mockClusterRepo{
				getFunc: func(ctx context.Context, id string) (*model.Cluster, error) {
					return cluster, nil
				},
			},
		}

		port := &mockNodePoolPort{
			deleteFunc: func(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error {
				return nil
			},
		}

		uc := &UseCase{Repos: repos, NodePoolPort: port}
		_, err := uc.Delete(ctx, &DeleteInput{ClusterID: clusterID, Name: "pool1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Delete(ctx, nil)
		if err == nil {
			t.Error("expected error for nil input")
		}
	})

	t.Run("empty cluster ID", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Delete(ctx, &DeleteInput{Name: "pool1"})
		if err == nil {
			t.Error("expected error for empty cluster ID")
		}
	})

	t.Run("empty pool name", func(t *testing.T) {
		uc := &UseCase{}
		_, err := uc.Delete(ctx, &DeleteInput{ClusterID: "ws1/prv1/cls1"})
		if err == nil {
			t.Error("expected error for empty pool name")
		}
	})
}
