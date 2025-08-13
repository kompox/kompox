package model

import "time"

// Cluster represents a Kubernetes cluster resource.
type Cluster struct {
	ID         string
	Name       string
	ProviderID string // references Provider
	Existing   bool
	Domain     string
	Ingress    map[string]interface{}
	Settings   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
