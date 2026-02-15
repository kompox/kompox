package rdb

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
	"gorm.io/gorm"
)

type BoxRepository struct{ db *gorm.DB }

func NewBoxRepository(db *gorm.DB) *BoxRepository { return &BoxRepository{db: db} }

func boxToRecord(b *model.Box) (*BoxRecord, error) {
	rec := &BoxRecord{
		ID:        b.ID,
		Name:      b.Name,
		AppID:     b.AppID,
		Component: b.Component,
		Image:     b.Image,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}

	if len(b.Command) > 0 {
		cmdJSON, err := json.Marshal(b.Command)
		if err != nil {
			return nil, err
		}
		rec.Command = string(cmdJSON)
	}

	if len(b.Args) > 0 {
		argsJSON, err := json.Marshal(b.Args)
		if err != nil {
			return nil, err
		}
		rec.Args = string(argsJSON)
	}

	if len(b.NetworkPolicy.IngressRules) > 0 {
		npJSON, err := json.Marshal(b.NetworkPolicy)
		if err != nil {
			return nil, err
		}
		rec.NetworkPolicy = string(npJSON)
	}

	return rec, nil
}

func boxToModel(r *BoxRecord) (*model.Box, error) {
	b := &model.Box{
		ID:        r.ID,
		Name:      r.Name,
		AppID:     r.AppID,
		Component: r.Component,
		Image:     r.Image,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}

	if r.Command != "" {
		if err := json.Unmarshal([]byte(r.Command), &b.Command); err != nil {
			return nil, err
		}
	}

	if r.Args != "" {
		if err := json.Unmarshal([]byte(r.Args), &b.Args); err != nil {
			return nil, err
		}
	}

	if r.NetworkPolicy != "" {
		if err := json.Unmarshal([]byte(r.NetworkPolicy), &b.NetworkPolicy); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (r *BoxRepository) Create(ctx context.Context, b *model.Box) error {
	rec, err := boxToRecord(b)
	if err != nil {
		return err
	}
	if rec.ID == "" {
		rec.ID = "box-" + uuid.NewString()
		b.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *BoxRepository) Get(ctx context.Context, id string) (*model.Box, error) {
	var rec BoxRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrBoxNotFound
		}
		return nil, err
	}
	return boxToModel(&rec)
}

func (r *BoxRepository) List(ctx context.Context) ([]*model.Box, error) {
	var recs []BoxRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Box, 0, len(recs))
	for i := range recs {
		box, err := boxToModel(&recs[i])
		if err != nil {
			return nil, err
		}
		out = append(out, box)
	}
	return out, nil
}

func (r *BoxRepository) ListByAppID(ctx context.Context, appID string) ([]*model.Box, error) {
	var recs []BoxRecord
	if err := r.db.WithContext(ctx).Where("app_id = ?", appID).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Box, 0, len(recs))
	for i := range recs {
		box, err := boxToModel(&recs[i])
		if err != nil {
			return nil, err
		}
		out = append(out, box)
	}
	return out, nil
}

func (r *BoxRepository) Update(ctx context.Context, b *model.Box) error {
	rec, err := boxToRecord(b)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&BoxRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *BoxRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&BoxRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrBoxNotFound
	}
	return nil
}

var _ domain.BoxRepository = (*BoxRepository)(nil)
