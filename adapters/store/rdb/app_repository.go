package rdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
	"gorm.io/gorm"
)

type AppRepository struct{ db *gorm.DB }

func NewAppRepository(db *gorm.DB) *AppRepository { return &AppRepository{db: db} }

func appToRecord(a *model.App) *AppRecord {
	return &AppRecord{ID: a.ID, Name: a.Name, ClusterID: a.ClusterID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
func appToModel(r *AppRecord) *model.App {
	return &model.App{ID: r.ID, Name: r.Name, ClusterID: r.ClusterID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r *AppRepository) Create(ctx context.Context, a *model.App) error {
	rec := appToRecord(a)
	if rec.ID == "" {
		rec.ID = "app-" + uuid.NewString()
		a.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *AppRepository) Get(ctx context.Context, id string) (*model.App, error) {
	var rec AppRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrAppNotFound
		}
		return nil, err
	}
	return appToModel(&rec), nil
}

func (r *AppRepository) List(ctx context.Context) ([]*model.App, error) {
	var recs []AppRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.App, 0, len(recs))
	for i := range recs {
		out = append(out, appToModel(&recs[i]))
	}
	return out, nil
}

func (r *AppRepository) Update(ctx context.Context, a *model.App) error {
	rec := appToRecord(a)
	return r.db.WithContext(ctx).Model(&AppRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *AppRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&AppRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrAppNotFound
	}
	return nil
}

var _ domain.AppRepository = (*AppRepository)(nil)
