package v1alpha1

import (
	"fmt"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

// FQN (Fully Qualified Name) represents the canonical identifier for each Kind.
// Format by Kind:
//   - Workspace: "ws"
//   - Provider: "ws/prv"
//   - Cluster: "ws/prv/cls"
//   - App: "ws/prv/cls/app"
//   - Box: "ws/prv/cls/app/box"
type FQN string

// String returns the FQN as a string.
func (f FQN) String() string {
	return string(f)
}

// Segments returns the path segments.
func (f FQN) Segments() []string {
	if f == "" {
		return nil
	}
	return strings.Split(string(f), "/")
}

// WorkspaceName returns the workspace segment.
func (f FQN) WorkspaceName() string {
	segs := f.Segments()
	if len(segs) > 0 {
		return segs[0]
	}
	return ""
}

// ProviderName returns the provider segment (if exists).
func (f FQN) ProviderName() string {
	segs := f.Segments()
	if len(segs) > 1 {
		return segs[1]
	}
	return ""
}

// ClusterName returns the cluster segment (if exists).
func (f FQN) ClusterName() string {
	segs := f.Segments()
	if len(segs) > 2 {
		return segs[2]
	}
	return ""
}

// AppName returns the app segment (if exists).
func (f FQN) AppName() string {
	segs := f.Segments()
	if len(segs) > 3 {
		return segs[3]
	}
	return ""
}

// BoxName returns the box segment (if exists).
func (f FQN) BoxName() string {
	segs := f.Segments()
	if len(segs) > 4 {
		return segs[4]
	}
	return ""
}

// ParentFQN returns the parent FQN (empty for Workspace).
func (f FQN) ParentFQN() FQN {
	segs := f.Segments()
	if len(segs) <= 1 {
		return ""
	}
	return FQN(strings.Join(segs[:len(segs)-1], "/"))
}

// ValidateSegmentCount validates the number of segments for the given Kind.
func ValidateSegmentCount(kind string, path string) error {
	segs := strings.Split(path, "/")
	var expected int
	switch kind {
	case "Workspace":
		expected = 1
	case "Provider":
		expected = 2
	case "Cluster":
		expected = 3
	case "App":
		expected = 4
	case "Box":
		expected = 5
	default:
		return fmt.Errorf("unknown kind: %s", kind)
	}

	if len(segs) != expected {
		return fmt.Errorf("kind %s expects %d segments, got %d in path %q", kind, expected, len(segs), path)
	}
	return nil
}

// ValidateSegmentLabels validates that all segments are valid DNS-1123 labels.
func ValidateSegmentLabels(path string) error {
	segs := strings.Split(path, "/")
	for i, seg := range segs {
		if errs := utilvalidation.IsDNS1123Label(seg); len(errs) > 0 {
			return fmt.Errorf("segment %d (%q) in path %q: %s", i, seg, path, strings.Join(errs, ", "))
		}
	}
	return nil
}

// BuildFQN constructs an FQN from a parent path and name, validating the result.
func BuildFQN(kind string, parentPath string, name string) (FQN, error) {
	var fqn string
	switch kind {
	case "Workspace":
		fqn = name
	case "Provider", "Cluster", "App", "Box":
		if parentPath == "" {
			return "", fmt.Errorf("kind %s requires a parent path", kind)
		}
		fqn = parentPath + "/" + name
	default:
		return "", fmt.Errorf("unknown kind: %s", kind)
	}

	if err := ValidateSegmentCount(kind, fqn); err != nil {
		return "", err
	}
	if err := ValidateSegmentLabels(fqn); err != nil {
		return "", err
	}

	return FQN(fqn), nil
}

// ExtractParentPath extracts the parent path from annotations for the given Kind.
// Returns empty string for Workspace (no parent).
func ExtractParentPath(kind string, annotations map[string]string) (string, error) {
	if kind == "Workspace" {
		return "", nil
	}

	path, ok := annotations[AnnotationPath]
	if !ok {
		return "", fmt.Errorf("kind %s requires annotation %q", kind, AnnotationPath)
	}
	return path, nil
}
