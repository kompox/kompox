package rdb

import "time"

// ServiceRecord is the RDB persistence model for domain Service.
// Table name: services
type ServiceRecord struct {
	ID         string    `gorm:"primaryKey;type:text;not null"`
	Name       string    `gorm:"type:text;not null"`
	ProviderID string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (ServiceRecord) TableName() string { return "services" }

// ProviderRecord persistence model
type ProviderRecord struct {
	ID        string    `gorm:"primaryKey;type:text;not null"`
	Name      string    `gorm:"type:text;not null"`
	Driver    string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (ProviderRecord) TableName() string { return "providers" }

// ClusterRecord persistence model
type ClusterRecord struct {
	ID         string    `gorm:"primaryKey;type:text;not null"`
	Name       string    `gorm:"type:text;not null"`
	ProviderID string    `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (ClusterRecord) TableName() string { return "clusters" }

// AppRecord persistence model
type AppRecord struct {
	ID        string    `gorm:"primaryKey;type:text;not null"`
	Name      string    `gorm:"type:text;not null"`
	ClusterID string    `gorm:"type:text"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (AppRecord) TableName() string { return "apps" }
