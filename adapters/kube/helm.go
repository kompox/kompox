package kube

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// HelmValues represents Helm chart values as a generic map.
// Keep it simple to interop with Helm SDK (yaml.Values).
type HelmValues map[string]any

// HelmValuesMutator can modify values for a specific release.
// Implementations should be deterministic and avoid side effects outside 'values'.
type HelmValuesMutator func(ctx context.Context, cluster *model.Cluster, release string, values HelmValues)
