package model

import (
	"fmt"
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

// ClusterStatusProvider is an interface for getting cluster status from a provider driver.
type ClusterStatusProvider interface {
	GetClusterStatus(cluster *Cluster) (*ClusterStatus, error)
}

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing    bool `json:"existing"`    // cluster.existing の設定値
	Provisioned bool `json:"provisioned"` // K8s クラスタが存在するとき true
	Installed   bool `json:"installed"`   // K8s クラスタ内のリソースが存在するとき true
}

// GetStatus returns the status of the cluster using the provided status provider.
func (c *Cluster) GetStatus(provider ClusterStatusProvider) (*ClusterStatus, error) {
	if provider == nil {
		return nil, fmt.Errorf("status provider is required")
	}
	return provider.GetClusterStatus(c)
}
