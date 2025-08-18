package app

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/internal/compose"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidateInput represents parameters to Validate.
type ValidateInput struct {
	ID string
}

// ValidateOutput represents result of validation.
type ValidateOutput struct {
	Errors     []string
	Warnings   []string
	Compose    string           // normalized compose YAML (if valid)
	K8sObjects []runtime.Object // converted Kubernetes objects (nil if conversion failed)
}

// Validate checks the compose string stored in App resource is valid YAML.
// Future enhancements: semantic checks, Kompose transform, policy checks.
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

	pro, err := compose.NewProject(ctx, a.Compose)
	if err != nil {
		return out, fmt.Errorf("compose project failed: %w", err)
	}

	b, err := pro.MarshalYAML()
	if err != nil {
		return out, fmt.Errorf("failed to marshal project to YAML: %w", err)
	}
	out.Compose = string(b)

	if u.KubeConverter != nil {
		objs, warns, err := u.KubeConverter.ComposeToObjects(ctx, []byte(out.Compose), kube.ConvertOption{Replicas: 1, Controller: "deployment", WithAnnotations: false})
		if err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("kompose conversion failed: %v", err))
		} else {
			out.K8sObjects = objs
			out.Warnings = append(out.Warnings, warns...)
		}
	}

	return out, nil
}
