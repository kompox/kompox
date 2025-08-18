package app

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidateInput represents parameters to Validate.

// ValidateOutput represents result of validation.
type ValidateOutput struct {
	Errors     []string
	Warnings   []string
	Compose    string           // normalized compose YAML (if valid)
	K8sObjects []runtime.Object // converted Kubernetes objects (nil if conversion failed)
}

// Validate checks the compose string stored in App resource is valid YAML.
// Future enhancements: semantic checks, policy checks.
func (u *UseCase) Validate(ctx context.Context, in ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in.ID == "" {
		return out, fmt.Errorf("missing app ID")
	}

	a, err := u.Repos.App.Get(ctx, in.ID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if a == nil {
		return out, fmt.Errorf("app not found: %s", in.ID)
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
