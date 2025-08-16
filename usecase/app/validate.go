package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	komposeapp "github.com/kubernetes/kompose/pkg/app"
	"github.com/kubernetes/kompose/pkg/kobject"
	"gopkg.in/yaml.v3"
)

// ValidateInput represents parameters to Validate.
type ValidateInput struct {
	ID string
}

// ValidateOutput represents result of validation.
type ValidateOutput struct {
	Errors      []string
	Warnings    []string
	Compose     string // normalized compose YAML (if valid)
	Raw         string // original compose string
	K8sManifest string // generated multi-document Kubernetes manifest (--- separated)
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
	out.Compose = string(normalizedBytes)

	// Basic structural checks
	if m, ok := generic.(map[string]any); ok {
		if _, ok := m["services"]; !ok {
			out.Warnings = append(out.Warnings, "top-level 'services' key not found (required for docker compose v2 style)")
		}
	} else {
		out.Errors = append(out.Errors, "top-level YAML must be a mapping object")
	}
	// Attempt Kompose conversion to Kubernetes manifests.
	// We rely on Kompose CLI internal API by writing compose content to a temp file and capturing a single output file.
	tmpDir, err := os.MkdirTemp("", "kompoxops-compose-*")
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("unable to create temp dir for kompose conversion: %v", err))
		return out, nil
	}
	// Best-effort cleanup.
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "compose.yaml")
	if err := os.WriteFile(composePath, []byte(composeStr), 0o600); err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("unable to write temp compose file: %v", err))
		return out, nil
	}
	manifestPath := filepath.Join(tmpDir, "k8s.yaml")
	// Prepare options (single multi-doc file output)
	opt := kobject.ConvertOptions{
		InputFiles: []string{composePath},
		OutFile:    manifestPath,
		Provider:   "kubernetes",
		// Other defaults: YAMLIndent left zero -> kompose default (2)
	}
	// Perform conversion (side-effect: writes manifestPath)
	// komposeapp.Convert exits process (log.Fatalf) on some errors; we can't easily intercept.
	// We assume well-formed compose to avoid that; still wrap recover just in case.
	func() {
		defer func() {
			if r := recover(); r != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("kompose conversion panicked: %v", r))
			}
		}()
		if _, err := komposeapp.Convert(opt); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("kompose conversion failed: %v", err))
		}
	}()
	if b, err := os.ReadFile(manifestPath); err == nil {
		out.K8sManifest = string(b)
	}

	return out, nil
}
