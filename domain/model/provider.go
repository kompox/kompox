package model

import "time"

// Provider represents an infrastructure provider (e.g., AKS, k3s).
type Provider struct {
	ID          string
	Name        string
	WorkspaceID string // references Workspace
	Driver      string // e.g., "aks", "k3s"
	Settings    map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
