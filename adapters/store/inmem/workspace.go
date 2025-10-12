package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// WorkspaceRepository is a thread-safe in-memory implementation.
type WorkspaceRepository struct {
	mu         sync.RWMutex
	workspaces map[string]*model.Workspace
	seq        int64
}

func NewWorkspaceRepository() *WorkspaceRepository {
	return &WorkspaceRepository{
		workspaces: make(map[string]*model.Workspace),
	}
}

func (r *WorkspaceRepository) nextID() string {
	r.seq++
	return fmt.Sprintf("ws-%d-%d", time.Now().UnixNano(), r.seq)
}

func (r *WorkspaceRepository) Create(_ context.Context, s *model.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s.ID == "" {
		s.ID = r.nextID()
	}
	// Copy to avoid external mutation.
	cp := *s
	r.workspaces[s.ID] = &cp
	return nil
}

func (r *WorkspaceRepository) Get(_ context.Context, id string) (*model.Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.workspaces[id]
	if !ok {
		return nil, model.ErrWorkspaceNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *WorkspaceRepository) List(_ context.Context) ([]*model.Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Workspace, 0, len(r.workspaces))
	for _, v := range r.workspaces {
		cp := *v
		out = append(out, &cp)
	}
	return out, nil
}

func (r *WorkspaceRepository) Update(_ context.Context, s *model.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.workspaces[s.ID]
	if !ok {
		return model.ErrWorkspaceNotFound
	}
	cp := *s
	// Preserve CreatedAt if caller accidentally changed it.
	cp.CreatedAt = existing.CreatedAt
	r.workspaces[s.ID] = &cp
	return nil
}

func (r *WorkspaceRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[id]; !ok {
		return model.ErrWorkspaceNotFound
	}
	delete(r.workspaces, id)
	return nil
}

// Compile-time assertion.
var _ domain.WorkspaceRepository = (*WorkspaceRepository)(nil)
