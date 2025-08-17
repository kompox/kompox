package kube

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
)

// ConvertOption defines generic options for compose -> Kubernetes object conversion.
type ConvertOption struct {
	Profiles        []string
	Replicas        int
	Controller      string // e.g. "deployment"
	WithAnnotations bool
}

// Converter converts a (normalized) docker compose YAML to Kubernetes runtime objects.
// Returns objects, warnings (non-fatal), and error.
type Converter interface {
	ComposeToObjects(ctx context.Context, composeYAML []byte, opt ConvertOption) ([]runtime.Object, []string, error)
}
