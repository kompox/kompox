package model

import (
	"time"
)

// Box represents a deployable unit (component) under an App.
// Box can be either:
//   - Compose Box: derived from App.spec.compose services (Image is empty)
//   - Standalone Box: independent image-based workload (Image is present)
type Box struct {
	ID            string
	Name          string
	AppID         string // references App
	Component     string // componentName (defaults to Name if empty)
	Image         string // if present, this is a Standalone Box
	Command       []string
	Args          []string
	NetworkPolicy BoxNetworkPolicy
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BoxNetworkPolicy defines network policy configuration for a Box.
type BoxNetworkPolicy struct {
	IngressRules []AppNetworkPolicyIngressRule
}

// ComponentName returns the effective component name for this Box.
// If Component is set, it is returned; otherwise, Name is returned.
func (b *Box) ComponentName() string {
	if b.Component != "" {
		return b.Component
	}
	return b.Name
}

// IsStandalone returns true if this is a Standalone Box (Image is present).
func (b *Box) IsStandalone() bool {
	return b.Image != ""
}

// IsCompose returns true if this is a Compose Box (Image is absent).
func (b *Box) IsCompose() bool {
	return b.Image == ""
}
