package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID        string
	Name      string
	ClusterID string // references Cluster
	Compose   string
	Ingress   []AppIngressRule
	Volumes   []AppVolume
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

// AppVolume defines a persistent volume requested by the app.
// Size is stored in bytes (parsed from user configuration quantities like "32Gi").
type AppVolume struct {
	Name string
	Size int64 // bytes
}

// AppVolumeInstance
type AppVolumeInstance struct {
	Name       string // name of the volume instance
	VolumeName string // name of the volume this instance is based on
	Assigned   bool   // whether this instance is assigned to the volume
	Size       int64  // volume instance size in bytes
	Handle     string // provider driver specific handle
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
