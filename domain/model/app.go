package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID        string
	Name      string
	ClusterID string
	CreatedAt time.Time
	UpdatedAt time.Time
}
