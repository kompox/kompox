package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID        string
	Name      string
	ClusterID string // references Cluster
	Compose   string
	Ingress   map[string]string
	Resources map[string]string
	Settings  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}
