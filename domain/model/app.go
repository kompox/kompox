package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID        string
	Name      string
	ClusterID string // references Cluster
	Compose   string
	Ingress   AppIngress
	Volumes   []AppVolume
	Resources map[string]string
	Settings  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AppIngress defines ingress-wide settings and rules for an app.
type AppIngress struct {
	// CertResolver overrides cluster-level resolver when set.
	CertResolver string
	Rules        []AppIngressRule
}

// AppIngressRule defines external exposure of a host/port.
type AppIngressRule struct {
	Name  string
	Port  int
	Hosts []string
}

// AppVolume defines a persistent volume requested by the app.
type AppVolume struct {
	Name string
	Size int64 // in bytes (parsed from user configuration quantities like "32Gi").
}

// AppVolumeInstance represents a specific instance of a volume.
type AppVolumeInstance struct {
	Name       string    `json:"name"`       // name of the volume instance
	VolumeName string    `json:"volumeName"` // name of the volume this instance is based on
	Assigned   bool      `json:"assigned"`   // whether this instance is assigned to the volume
	Size       int64     `json:"size"`       // volume instance size in bytes
	Handle     string    `json:"handle"`     // provider driver specific handle
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
