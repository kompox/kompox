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

// Validate validates documents in topological order.
// It checks:
//  1. Kind and Resource ID segment count consistency
//  2. Duplicate Resource IDs within the document set
//  3. Parent existence (except for Workspace)
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
