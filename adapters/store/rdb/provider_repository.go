package rdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
	"gorm.io/gorm"
)

// ProviderRepository is a GORM-backed implementation of domain.ProviderRepository.
type ProviderRepository struct{ db *gorm.DB }

func NewProviderRepository(db *gorm.DB) *ProviderRepository { return &ProviderRepository{db: db} }

func providerToRecord(p *model.Provider) *ProviderRecord {
	return &ProviderRecord{ID: p.ID, Name: p.Name, Driver: p.Driver, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
}
func providerToModel(r *ProviderRecord) *model.Provider {
	return &model.Provider{ID: r.ID, Name: r.Name, Driver: r.Driver, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r *ProviderRepository) Create(ctx context.Context, p *model.Provider) error {
	rec := providerToRecord(p)
	if rec.ID == "" {
		rec.ID = "prov-" + uuid.NewString()
		p.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *ProviderRepository) Get(ctx context.Context, id string) (*model.Provider, error) {
	var rec ProviderRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrProviderNotFound
		}
		return nil, err
	}
	return providerToModel(&rec), nil
}

func (r *ProviderRepository) List(ctx context.Context) ([]*model.Provider, error) {
	var recs []ProviderRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Provider, 0, len(recs))
	for i := range recs {
		out = append(out, providerToModel(&recs[i]))
	}
	return out, nil
}

func (r *ProviderRepository) Update(ctx context.Context, p *model.Provider) error {
	rec := providerToRecord(p)
	return r.db.WithContext(ctx).Model(&ProviderRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *ProviderRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&ProviderRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrProviderNotFound
	}
	return nil
}

var _ domain.ProviderRepository = (*ProviderRepository)(nil)
