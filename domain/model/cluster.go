package model

import "time"

// Cluster represents a Kubernetes cluster resource.
type Cluster struct {
	ID         string
	Name       string
	ProviderID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
