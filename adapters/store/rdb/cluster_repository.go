package rdb

import (
	"context"

	"github.com/google/uuid"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
	"gorm.io/gorm"
)

type ClusterRepository struct{ db *gorm.DB }

func NewClusterRepository(db *gorm.DB) *ClusterRepository { return &ClusterRepository{db: db} }

func clusterToRecord(c *model.Cluster) *ClusterRecord {
	return &ClusterRecord{ID: c.ID, Name: c.Name, ProviderID: c.ProviderID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}
func clusterToModel(r *ClusterRecord) *model.Cluster {
	return &model.Cluster{ID: r.ID, Name: r.Name, ProviderID: r.ProviderID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (r *ClusterRepository) Create(ctx context.Context, c *model.Cluster) error {
	rec := clusterToRecord(c)
	if rec.ID == "" {
		rec.ID = "clus-" + uuid.NewString()
		c.ID = rec.ID
	}
	return r.db.WithContext(ctx).Create(rec).Error
}

func (r *ClusterRepository) Get(ctx context.Context, id string) (*model.Cluster, error) {
	var rec ClusterRecord
	if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, model.ErrClusterNotFound
		}
		return nil, err
	}
	return clusterToModel(&rec), nil
}

func (r *ClusterRepository) List(ctx context.Context) ([]*model.Cluster, error) {
	var recs []ClusterRecord
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&recs).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Cluster, 0, len(recs))
	for i := range recs {
		out = append(out, clusterToModel(&recs[i]))
	}
	return out, nil
}

func (r *ClusterRepository) Update(ctx context.Context, c *model.Cluster) error {
	rec := clusterToRecord(c)
	return r.db.WithContext(ctx).Model(&ClusterRecord{}).Where("id = ?", rec.ID).Updates(rec).Error
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&ClusterRecord{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return model.ErrClusterNotFound
	}
	return nil
}

var _ domain.ClusterRepository = (*ClusterRepository)(nil)
