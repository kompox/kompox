package app

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/adapters/kube"
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
	// Issues contains structured severity information.
	Issues []Issue
	// Compose is the normalized compose YAML when validation succeeds.
	Compose string
	// Converter is the populated kube.Converter when conversion succeeds (nil otherwise).
	Converter *kube.Converter
	// K8sObjects are generated Kubernetes manifests if conversion succeeds.
	K8sObjects []runtime.Object
}

// Validate checks the compose string stored in an App resource.
// It performs syntactic validation and best-effort conversion to Kubernetes objects.
func (u *UseCase) Validate(ctx context.Context, in *ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in == nil || in.AppID == "" {
		return out, fmt.Errorf("missing app ID")
	}

	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if app == nil {
		return out, fmt.Errorf("app not found: %s", in.AppID)
	}

	res, err := u.validateApp(ctx, app)
	if err != nil {
		return out, err
	}
	if res == nil {
		return out, fmt.Errorf("validation result unavailable")
	}
	out.Compose = res.Compose
	out.Converter = res.Converter
	out.K8sObjects = res.K8sObjects
	out.Issues = append(out.Issues, res.Issues...)
	for _, issue := range res.Issues {
		switch issue.Severity {
		case SeverityError:
			out.Errors = append(out.Errors, issue.Message)
		default:
			out.Warnings = append(out.Warnings, issue.Message)
		}
	}
	return out, nil
}
