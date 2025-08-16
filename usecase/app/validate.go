package app

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateInput represents parameters to Validate.
type ValidateInput struct {
	ID string
}

// ValidateOutput represents result of validation.
type ValidateOutput struct {
	Errors     []string
	Warnings   []string
	Normalized string // normalized compose YAML (if valid)
	Raw        string // original compose string
}

// Validate checks the compose string stored in App resource is valid YAML.
// Future enhancements: semantic checks, Kompose transform, policy checks.
func (u *UseCase) Validate(ctx context.Context, in ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in.ID == "" {
		out.Errors = append(out.Errors, "missing app ID")
		return out, nil
	}
	a, err := u.Repos.App.Get(ctx, in.ID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if a == nil {
		out.Errors = append(out.Errors, fmt.Sprintf("app not found: %s", in.ID))
		return out, nil
	}
	composeStr := a.Compose
	out.Raw = composeStr
	if composeStr == "" {
		out.Errors = append(out.Errors, "compose definition empty")
		return out, nil
	}

	var generic any
	if err := yaml.Unmarshal([]byte(composeStr), &generic); err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("invalid YAML: %v", err))
		return out, nil
	}

	// Normalize
	normalizedBytes, err := yaml.Marshal(generic)
	if err != nil {
		out.Errors = append(out.Errors, fmt.Sprintf("failed to normalize YAML: %v", err))
		return out, nil
	}
	out.Normalized = string(normalizedBytes)

	// Basic structural checks
	if m, ok := generic.(map[string]any); ok {
		if _, ok := m["services"]; !ok {
			out.Warnings = append(out.Warnings, "top-level 'services' key not found (required for docker compose v2 style)")
		}
	} else {
		out.Errors = append(out.Errors, "top-level YAML must be a mapping object")
	}
	return out, nil
}
