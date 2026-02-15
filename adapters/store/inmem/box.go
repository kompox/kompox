package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// BoxRepository is a thread-safe in-memory implementation.
type BoxRepository struct {
	mu    sync.RWMutex
	items map[string]*model.Box
	seq   int64
}

func NewBoxRepository() *BoxRepository {
	return &BoxRepository{items: make(map[string]*model.Box)}
}

func (r *BoxRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("box-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *BoxRepository) Create(_ context.Context, b *model.Box) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b.ID == "" {
		b.ID = r.nextID()
	}
	cp := *b
	r.items[b.ID] = &cp
	return nil
}

func (r *BoxRepository) Get(_ context.Context, id string) (*model.Box, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[id]
	if !ok {
		return nil, model.ErrBoxNotFound
	}
	cp := *v
	return &cp, nil
}

func (r *BoxRepository) List(_ context.Context) ([]*model.Box, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Box, 0, len(r.items))
	for _, v := range r.items {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *BoxRepository) ListByAppID(_ context.Context, appID string) ([]*model.Box, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Box, 0)
	for _, v := range r.items {
		if v.AppID == appID {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *BoxRepository) Update(_ context.Context, b *model.Box) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.items[b.ID]
	if !ok {
		return model.ErrBoxNotFound
	}
	cp := *b
	r.items[b.ID] = &cp
	return nil
}

func (r *BoxRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return model.ErrBoxNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.BoxRepository = (*BoxRepository)(nil)
