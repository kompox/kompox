package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// InMemoryAppRepository is a thread-safe in-memory implementation.
type InMemoryAppRepository struct {
	mu    sync.RWMutex
	items map[string]*model.App
	seq   int64
}

func NewInMemoryAppRepository() *InMemoryAppRepository {
	return &InMemoryAppRepository{items: make(map[string]*model.App)}
}

func (r *InMemoryAppRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("app-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *InMemoryAppRepository) Create(_ context.Context, a *model.App) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == "" {
		a.ID = r.nextID()
	}
	cp := *a
	r.items[a.ID] = &cp
	return nil
}

func (r *InMemoryAppRepository) Get(_ context.Context, id string) (*model.App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	if !ok {
		return nil, model.ErrAppNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *InMemoryAppRepository) List(_ context.Context) ([]*model.App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.App, 0, len(r.items))
	for _, v := range r.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *InMemoryAppRepository) Update(_ context.Context, a *model.App) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.items[a.ID]
	if !ok {
		return model.ErrAppNotFound
	}
	cp := *a
	r.items[a.ID] = &cp
	return nil
}

func (r *InMemoryAppRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrAppNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.AppRepository = (*InMemoryAppRepository)(nil)
