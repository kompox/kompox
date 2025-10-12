package rdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
	"gorm.io/gorm"
)

// WorkspaceRepository is a GORM-backed implementation of domain.WorkspaceRepository.
type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func toRecord(s *model.Workspace) *WorkspaceRecord {
	return &WorkspaceRecord{
		ID:        s.ID,
		Name:      s.Name,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func toModel(r *WorkspaceRecord) *model.Workspace {
	return &model.Workspace{
		ID:        r.ID,
		Name:      r.Name,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func (r *WorkspaceRepository) Create(ctx context.Context, s *model.Workspace) error {
	rec := toRecord(s)
	if rec.ID == "" {
		// Generate a unique ID if not provided
		rec.ID = "ws-" + uuid.NewString()
		s.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *WorkspaceRepository) Get(ctx context.Context, id string) (*model.Workspace, error) {
	var rec WorkspaceRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrWorkspaceNotFound
		}
		return nil, err
	}
	return toModel(&rec), nil
}

func (r *WorkspaceRepository) List(ctx context.Context) ([]*model.Workspace, error) {
	var recs []WorkspaceRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Workspace, 0, len(recs))
	for i := range recs {
		out = append(out, toModel(&recs[i]))
	}
	return out, nil
}

func (r *WorkspaceRepository) Update(ctx context.Context, s *model.Workspace) error {
	rec := toRecord(s)
	return r.db.WithContext(ctx).Model(&WorkspaceRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&WorkspaceRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrWorkspaceNotFound
	}
	return nil
}

// Ensure interface satisfaction.
var _ domain.WorkspaceRepository = (*WorkspaceRepository)(nil)
