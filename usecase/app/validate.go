package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubernetes/kompose/pkg/kobject"
	"github.com/kubernetes/kompose/pkg/loader"
	"github.com/kubernetes/kompose/pkg/transformer/kubernetes"
	"gopkg.in/yaml.v3"
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
	Raw        string           // original compose string
	K8sObjects []runtime.Object // converted Kubernetes objects (nil if conversion failed)
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

	tmpDir, err := os.MkdirTemp("", "kompoxops-kompose-*")
	if err != nil {
		return out, fmt.Errorf("failed to create temp dir for kompose: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "compose.yaml")
	err = os.WriteFile(composePath, normalizedBytes, 0o600)
	if err != nil {
		return out, fmt.Errorf("write temp compose file failed: %w", err)
	}

	kLoader, err := loader.GetLoader("compose")
	if err != nil {
		return out, fmt.Errorf("failed to get kompose loader: %w", err)
	}
	profiles := []string{}
	kObjects, err := kLoader.LoadFile([]string{composePath}, profiles)
	if err != nil {
		return out, fmt.Errorf("kompose load file failed: %w", err)
	}

	convertOptions := kobject.ConvertOptions{
		Provider:   "kubernetes",
		Controller: "deployment",
		Replicas:   1,
		YAMLIndent: 2,
		Profiles:   profiles,
	}
	kTransformer := &kubernetes.Kubernetes{}
	rObjects, err := kTransformer.Transform(kObjects, convertOptions)
	if err != nil {
		return out, fmt.Errorf("transform failed: %w", err)
	}

	out.K8sObjects = rObjects

	return out, nil
}
