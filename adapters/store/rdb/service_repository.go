package rdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
	"gorm.io/gorm"
)

// ServiceRepository is a GORM-backed implementation of domain.ServiceRepository.
type ServiceRepository struct {
	db *gorm.DB
}

func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

func toRecord(s *model.Service) *ServiceRecord {
	return &ServiceRecord{
		ID:         s.ID,
		Name:       s.Name,
		ProviderID: s.ProviderID,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func toModel(r *ServiceRecord) *model.Service {
	return &model.Service{
		ID:         r.ID,
		Name:       r.Name,
		ProviderID: r.ProviderID,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func (r *ServiceRepository) Create(ctx context.Context, s *model.Service) error {
	rec := toRecord(s)
	if rec.ID == "" {
		// Generate a unique ID if not provided
		rec.ID = "svc-" + uuid.NewString()
		s.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *ServiceRepository) Get(ctx context.Context, id string) (*model.Service, error) {
	var rec ServiceRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrServiceNotFound
		}
		return nil, err
	}
	return toModel(&rec), nil
}

func (r *ServiceRepository) List(ctx context.Context) ([]*model.Service, error) {
	var recs []ServiceRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Service, 0, len(recs))
	for i := range recs {
		out = append(out, toModel(&recs[i]))
	}
	return out, nil
}

func (r *ServiceRepository) Update(ctx context.Context, s *model.Service) error {
	rec := toRecord(s)
	return r.db.WithContext(ctx).Model(&ServiceRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *ServiceRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&ServiceRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrServiceNotFound
	}
	return nil
}

// Ensure interface satisfaction.
var _ domain.ServiceRepository = (*ServiceRepository)(nil)
