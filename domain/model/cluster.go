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
	Ingress    *ClusterIngress
	Settings   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ClusterIngress defines cluster-level ingress configuration.
type ClusterIngress struct {
	Namespace      string
	Controller     string
	ServiceAccount string
}

// ClusterPort is an interface (domain port) for cluster operations.
type ClusterPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
	Provision(ctx context.Context, cluster *Cluster) error
	Deprovision(ctx context.Context, cluster *Cluster) error
	Install(ctx context.Context, cluster *Cluster) error
	Uninstall(ctx context.Context, cluster *Cluster) error
}

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing    bool `json:"existing"`    // Value of cluster.existing configuration
	Provisioned bool `json:"provisioned"` // True when the Kubernetes cluster exists
	Installed   bool `json:"installed"`   // True when in-cluster resources are installed
}

// Note: Status retrieval should be invoked from use cases through a ClusterPort.
