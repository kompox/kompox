package model

import (
	"context"
	"time"
)

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

// ClusterStatusPort is an interface (domain port) for obtaining cluster status.
type ClusterStatusPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
}

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing    bool `json:"existing"`    // Value of cluster.existing configuration
	Provisioned bool `json:"provisioned"` // True when the Kubernetes cluster exists
	Installed   bool `json:"installed"`   // True when in-cluster resources are installed
}

// Note: Status retrieval should be invoked from use cases through a ClusterStatusPort.
