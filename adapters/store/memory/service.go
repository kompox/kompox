package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// InMemoryServiceRepository is a thread-safe in-memory implementation.
type InMemoryServiceRepository struct {
	mu       sync.RWMutex
	services map[string]*model.Service
	seq      int64
}

func NewInMemoryServiceRepository() *InMemoryServiceRepository {
	return &InMemoryServiceRepository{
		services: make(map[string]*model.Service),
	}
}

func (r *InMemoryServiceRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("svc-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *InMemoryServiceRepository) Create(_ context.Context, s *model.Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == "" {
		s.ID = r.nextID()
	}
	// Copy to avoid external mutation.
	cp := *s
	r.services[s.ID] = &cp
	return nil
}

func (r *InMemoryServiceRepository) Get(_ context.Context, id string) (*model.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.services[id]
	if !ok {
		return nil, model.ErrServiceNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *InMemoryServiceRepository) List(_ context.Context) ([]*model.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Service, 0, len(r.services))
	for _, v := range r.services {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *InMemoryServiceRepository) Update(_ context.Context, s *model.Service) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.services[s.ID]
	if !ok {
		return model.ErrServiceNotFound
	}
	cp := *s
	// Preserve CreatedAt if caller accidentally changed it.
	cp.CreatedAt = existing.CreatedAt
	r.services[s.ID] = &cp
	return nil
}

func (r *InMemoryServiceRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.services[id]; !ok {
		return model.ErrServiceNotFound
	}
	delete(r.services, id)
	return nil
}

// Compile-time assertion.
var _ domain.ServiceRepository = (*InMemoryServiceRepository)(nil)
