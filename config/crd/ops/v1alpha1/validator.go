package v1alpha1

import (
	"fmt"
	"sort"
	"strings"
)

// ValidationError represents a validation error for a document.
type ValidationError struct {
	// FQN that was attempted to be built.
	FQN FQN
	// Kind of the document.
	Kind string
	// Error message.
	Message string
	// Path is the source file path where the validation error occurred.
	Path string
	// Index is the 1-based document position within the source file.
	Index int
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	location := ""
	if e.Path != "" {
		if e.Index > 0 {
			location = fmt.Sprintf(" from %s (document %d)", e.Path, e.Index)
		} else {
			location = fmt.Sprintf(" from %s", e.Path)
		}
	}
	return fmt.Sprintf("%s %q validation error: %s%s", strings.ToLower(e.Kind), e.FQN, e.Message, location)
}

// ValidationResult contains the results of validating documents.
type ValidationResult struct {
	// ValidDocuments are documents that passed all validation checks.
	ValidDocuments []Document
	// Errors are validation errors encountered.
	Errors []*ValidationError
}

// HasErrors returns true if there are any validation errors.
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// isDNS1123Label validates if a string is a valid DNS-1123 label.
// DNS-1123 label must:
// - contain at most 63 characters
// - contain only lowercase alphanumeric characters or '-'
// - start with an alphanumeric character
// - end with an alphanumeric character
func isDNS1123Label(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for i, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '-' && i > 0 && i < len(value)-1 {
			continue
		}
		return false
	}
	return true
}

// validateBox validates Box-specific constraints.
// Returns an error message if validation fails, empty string otherwise.
func validateBox(box *Box) string {
	// metadata.name must be a valid DNS-1123 label
	if !isDNS1123Label(box.ObjectMeta.Name) {
		return fmt.Sprintf("metadata.name %q is not a valid DNS-1123 label (must be lowercase alphanumeric with hyphens, max 63 chars)", box.ObjectMeta.Name)
	}

	// metadata.name must not be "app" (reserved word)
	if box.ObjectMeta.Name == "app" {
		return `metadata.name "app" is reserved and cannot be used for Box`
	}

	// spec.component, if specified, must match metadata.name
	if box.Spec.Component != "" && box.Spec.Component != box.ObjectMeta.Name {
		return fmt.Sprintf("spec.component %q must match metadata.name %q", box.Spec.Component, box.ObjectMeta.Name)
	}

	// Determine Box type based on spec.image
	isStandalone := box.Spec.Image != ""

	if isStandalone {
		// Standalone Box: spec.image is required (already checked above)
		// spec.ingress must not be specified (reserved)
		if box.Spec.Ingress != nil {
			return "Standalone Box must not specify spec.ingress (reserved for future use)"
		}
	} else {
		// Compose Box: spec.image/command/args/ingress must not be specified
		if box.Spec.Image != "" {
			return "Compose Box must not specify spec.image"
		}
		if len(box.Spec.Command) > 0 {
			return "Compose Box must not specify spec.command"
		}
		if len(box.Spec.Args) > 0 {
			return "Compose Box must not specify spec.args"
		}
		if box.Spec.Ingress != nil {
			return "Compose Box must not specify spec.ingress"
		}
	}

	return ""
}

// Validate validates documents in topological order.
// It checks:
//  1. Kind and Resource ID segment count consistency
//  2. Duplicate Resource IDs within the document set
//  3. Parent existence (except for Workspace)
//  4. Box-specific validation (metadata.name constraints, spec consistency)
//
// Returns ValidDocuments only if there are no errors.
func Validate(documents []Document) *ValidationResult {
	result := &ValidationResult{
		ValidDocuments: make([]Document, 0),
		Errors:         make([]*ValidationError, 0),
	}

	// Sort documents by topological order
	sorted := sortByTopology(documents)

	// Track FQNs seen in this validation batch
	seenFQNs := make(map[FQN]int) // FQN -> document index
	// Track FQNs that are valid parents (available for children)
	validParents := make(map[FQN]bool)

	for i, doc := range sorted {
		// Check Kind and Resource ID segment count consistency
		// The segment count is validated during parsing/FQN construction
		pairs := doc.FQN.KindSegments()
		expectedPairs := kindOrder(doc.Kind)
		if expectedPairs != 999 && len(pairs) != expectedPairs {
			result.Errors = append(result.Errors, &ValidationError{
				FQN:     doc.FQN,
				Kind:    doc.Kind,
				Message: fmt.Sprintf("kind %s expects %d kind/name pairs but Resource ID has %d", doc.Kind, expectedPairs, len(pairs)),
				Path:    doc.Path,
				Index:   doc.Index,
			})
			continue
		}

		// Check for duplicate Resource ID in this batch
		if prevIndex, exists := seenFQNs[doc.FQN]; exists {
			result.Errors = append(result.Errors, &ValidationError{
				FQN:     doc.FQN,
				Kind:    doc.Kind,
				Message: fmt.Sprintf("duplicate Resource ID, already defined at document %d", prevIndex),
				Path:    doc.Path,
				Index:   doc.Index,
			})
			continue
		}

		// Check parent existence (skip for Workspace)
		if doc.Kind != "Workspace" {
			parentFQN := doc.FQN.ParentFQN()
			if !validParents[parentFQN] {
				result.Errors = append(result.Errors, &ValidationError{
					FQN:     doc.FQN,
					Kind:    doc.Kind,
					Message: fmt.Sprintf("parent %q does not exist", parentFQN),
					Path:    doc.Path,
					Index:   doc.Index,
				})
				continue
			}
		}

		// Box-specific validation
		if doc.Kind == "Box" {
			if box, ok := doc.Object.(*Box); ok {
				if errMsg := validateBox(box); errMsg != "" {
					result.Errors = append(result.Errors, &ValidationError{
						FQN:     doc.FQN,
						Kind:    doc.Kind,
						Message: errMsg,
						Path:    doc.Path,
						Index:   doc.Index,
					})
					continue
				}
			}
		}

		// Document is valid
		seenFQNs[doc.FQN] = i
		validParents[doc.FQN] = true
		result.ValidDocuments = append(result.ValidDocuments, doc)
	}

	return result
}

// sortByTopology sorts documents by topological order:
// Workspace → Provider → Cluster → App → Box
// Within each level, sort alphabetically by Resource ID for deterministic order.
func sortByTopology(documents []Document) []Document {
	// Create a copy to avoid modifying the original
	sorted := make([]Document, len(documents))
	copy(sorted, documents)

	sort.Slice(sorted, func(i, j int) bool {
		// First, sort by kind order (Workspace < Provider < Cluster < App < Box)
		orderI := kindOrder(sorted[i].Kind)
		orderJ := kindOrder(sorted[j].Kind)
		if orderI != orderJ {
			return orderI < orderJ
		}

		// Within the same kind, sort alphabetically by FQN
		return sorted[i].FQN < sorted[j].FQN
	})

	return sorted
}

// kindOrder returns the topological order for a kind.
// Lower numbers come first.
func kindOrder(kind string) int {
	switch kind {
	case "Workspace":
		return 1
	case "Provider":
		return 2
	case "Cluster":
		return 3
	case "App":
		return 4
	case "Box":
		return 5
	default:
		return 999
	}
}
