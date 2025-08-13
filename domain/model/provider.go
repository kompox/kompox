package model

import "time"

// Provider represents an infrastructure provider (e.g., AKS, k3s).
type Provider struct {
	ID        string
	Name      string
	Driver    string // e.g., "aks", "k3s"
	CreatedAt time.Time
	UpdatedAt time.Time
}
