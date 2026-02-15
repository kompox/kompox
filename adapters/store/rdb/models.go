package rdb

import "time"

// WorkspaceRecord is the RDB persistence model for domain Workspace.
// Table name: workspaces
type WorkspaceRecord struct {
	ID        string    `gorm:"primaryKey;type:text;not null"`
	Name      string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (WorkspaceRecord) TableName() string { return "workspaces" }

// ProviderRecord persistence model
type ProviderRecord struct {
	ID          string    `gorm:"primaryKey;type:text;not null"`
	Name        string    `gorm:"type:text;not null"`
	WorkspaceID string    `gorm:"type:text;not null"` // references Workspace
	Driver      string    `gorm:"type:text;not null"`
	Settings    string    `gorm:"type:text"` // JSON encoded map[string]string
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (ProviderRecord) TableName() string { return "providers" }

// ClusterRecord persistence model
type ClusterRecord struct {
	ID         string    `gorm:"primaryKey;type:text;not null"`
	Name       string    `gorm:"type:text;not null"`
	ProviderID string    `gorm:"type:text;not null"` // references Provider
	Existing   bool      `gorm:"not null"`
	Domain     string    `gorm:"type:text"`
	Ingress    string    `gorm:"type:text"` // JSON encoded map[string]interface{}
	Settings   string    `gorm:"type:text"` // JSON encoded map[string]string
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (ClusterRecord) TableName() string { return "clusters" }

// AppRecord persistence model
type AppRecord struct {
	ID        string    `gorm:"primaryKey;type:text;not null"`
	Name      string    `gorm:"type:text;not null"`
	ClusterID string    `gorm:"type:text;not null"` // references Cluster
	Compose   string    `gorm:"type:text"`
	Ingress   string    `gorm:"type:text"` // JSON encoded map[string]string
	Resources string    `gorm:"type:text"` // JSON encoded map[string]string
	Settings  string    `gorm:"type:text"` // JSON encoded map[string]string
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (AppRecord) TableName() string { return "apps" }

// BoxRecord persistence model
type BoxRecord struct {
	ID            string    `gorm:"primaryKey;type:text;not null"`
	Name          string    `gorm:"type:text;not null"`
	AppID         string    `gorm:"type:text;not null"` // references App
	Component     string    `gorm:"type:text"`
	Image         string    `gorm:"type:text"`
	Command       string    `gorm:"type:text"` // JSON encoded []string
	Args          string    `gorm:"type:text"` // JSON encoded []string
	NetworkPolicy string    `gorm:"type:text"` // JSON encoded BoxNetworkPolicy
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (BoxRecord) TableName() string { return "boxes" }
