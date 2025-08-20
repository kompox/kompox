package app

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidateInput represents parameters to Validate.
// ValidateInput identifies the App whose compose specification should be validated.
type ValidateInput struct {
	// AppID is the application being validated.
	AppID string `json:"app_id"`
}

// ValidateOutput reports validation outcomes.
type ValidateOutput struct {
	// Errors are fatal validation failures.
	Errors []string
	// Warnings are non-fatal issues.
	Warnings []string
	// Compose is the normalized compose YAML when validation succeeds.
	Compose string // normalized compose YAML (if valid)
	// K8sObjects are generated Kubernetes manifests if conversion succeeds.
	K8sObjects []runtime.Object // converted Kubernetes objects (nil if conversion failed)
}

// Validate checks the compose string stored in an App resource.
// It performs syntactic validation and best-effort conversion to Kubernetes objects.
func (u *UseCase) Validate(ctx context.Context, in *ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in == nil || in.AppID == "" {
		return out, fmt.Errorf("missing app ID")
	}

	a, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if a == nil {
		return out, fmt.Errorf("app not found: %s", in.AppID)
	}

	pro, err := kube.ComposeAppToProject(ctx, a.Compose)
	if err != nil {
		return out, fmt.Errorf("compose project failed: %w", err)
	}

	b, err := pro.MarshalYAML()
	if err != nil {
		return out, fmt.Errorf("failed to marshal project to YAML: %w", err)
	}
	out.Compose = string(b)

	// Fetch related resources for hash & conversion
	cls, err := u.Repos.Cluster.Get(ctx, a.ClusterID)
	if err == nil && cls != nil {
		prv, _ := u.Repos.Provider.Get(ctx, cls.ProviderID)
		var svc *model.Service
		if prv != nil {
			svc, _ = u.Repos.Service.Get(ctx, prv.ServiceID)
		}
		if svc != nil && prv != nil {
			objs, warns, convErr := kube.ComposeAppToObjects(ctx, svc, prv, cls, a)
			if convErr != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: %v", convErr))
			} else {
				out.K8sObjects = objs
				out.Warnings = append(out.Warnings, warns...)
			}
		}
	}

	return out, nil
}
