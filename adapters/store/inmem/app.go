package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// AppRepository is a thread-safe in-memory implementation.
type AppRepository struct {
	mu    sync.RWMutex
	items map[string]*model.App
	seq   int64
}

func NewAppRepository() *AppRepository {
	return &AppRepository{items: make(map[string]*model.App)}
}

func (r *AppRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("app-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *AppRepository) Create(_ context.Context, a *model.App) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == "" {
		a.ID = r.nextID()
	}
	cp := *a
	r.items[a.ID] = &cp
	return nil
}

func (r *AppRepository) Get(_ context.Context, id string) (*model.App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	if !ok {
		return nil, model.ErrAppNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *AppRepository) List(_ context.Context) ([]*model.App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.App, 0, len(r.items))
	for _, v := range r.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *AppRepository) Update(_ context.Context, a *model.App) error {
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

func (r *AppRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrAppNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.AppRepository = (*AppRepository)(nil)
