package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// InMemoryProviderRepository is a thread-safe in-memory implementation.
type InMemoryProviderRepository struct {
	mu    sync.RWMutex
	items map[string]*model.Provider
	seq   int64
}

func NewInMemoryProviderRepository() *InMemoryProviderRepository {
	return &InMemoryProviderRepository{items: make(map[string]*model.Provider)}
}

func (r *InMemoryProviderRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("prov-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *InMemoryProviderRepository) Create(_ context.Context, p *model.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.ID == "" {
		p.ID = r.nextID()
	}
	cp := *p
	r.items[p.ID] = &cp
	return nil
}

func (r *InMemoryProviderRepository) Get(_ context.Context, id string) (*model.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	if !ok {
		return nil, model.ErrProviderNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *InMemoryProviderRepository) List(_ context.Context) ([]*model.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Provider, 0, len(r.items))
	for _, v := range r.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *InMemoryProviderRepository) Update(_ context.Context, p *model.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.items[p.ID]
	if !ok {
		return model.ErrProviderNotFound
	}
	cp := *p
	r.items[p.ID] = &cp
	return nil
}

func (r *InMemoryProviderRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrProviderNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.ProviderRepository = (*InMemoryProviderRepository)(nil)
