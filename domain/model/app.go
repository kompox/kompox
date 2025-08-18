package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID        string
	Name      string
	ClusterID string // references Cluster
	Compose   string
	Ingress   []AppIngressRule
	Resources map[string]string
	Settings  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AppIngressRule defines external exposure of a host/port.
type AppIngressRule struct {
	Name  string
	Port  int
	Hosts []string
}
