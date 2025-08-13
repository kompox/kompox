package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// ClusterRepository is a thread-safe in-memory implementation.
type ClusterRepository struct {
	mu    sync.RWMutex
	items map[string]*model.Cluster
	seq   int64
}

func NewClusterRepository() *ClusterRepository {
	return &ClusterRepository{items: make(map[string]*model.Cluster)}
}

func (r *ClusterRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("clus-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *ClusterRepository) Create(_ context.Context, c *model.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c.ID == "" {
		c.ID = r.nextID()
	}
	cp := *c
	r.items[c.ID] = &cp
	return nil
}

func (r *ClusterRepository) Get(_ context.Context, id string) (*model.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	if !ok {
		return nil, model.ErrClusterNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *ClusterRepository) List(_ context.Context) ([]*model.Cluster, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Cluster, 0, len(r.items))
	for _, v := range r.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *ClusterRepository) Update(_ context.Context, c *model.Cluster) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.items[c.ID]
	if !ok {
		return model.ErrClusterNotFound
	}
	cp := *c
	r.items[c.ID] = &cp
	return nil
}

func (r *ClusterRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrClusterNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.ClusterRepository = (*ClusterRepository)(nil)
