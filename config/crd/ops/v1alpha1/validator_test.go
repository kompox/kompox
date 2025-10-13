package v1alpha1

import (
	"testing"
)

func TestValidator_Validate_HappyPath(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "ws1/prv1/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app1", Object: &App{}},
		{Kind: "Box", FQN: "ws1/prv1/cls1/app1/box1", Object: &Box{}},
	}

	// validator removed
	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	if len(result.ValidDocuments) != 5 {
		t.Errorf("Validate() returned %d valid documents, want 5", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_OutOfOrder(t *testing.T) {
	// Documents in reverse topological order
	documents := []Document{
		{Kind: "Box", FQN: "ws1/prv1/cls1/app1/box1", Object: &Box{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app1", Object: &App{}},
		{Kind: "Cluster", FQN: "ws1/prv1/cls1", Object: &Cluster{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
	}

	// validator removed
	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	// Should still work because validator sorts them
	if len(result.ValidDocuments) != 5 {
		t.Errorf("Validate() returned %d valid documents, want 5", len(result.ValidDocuments))
	}

	// Check that they are sorted correctly
	expectedOrder := []string{"ws1", "ws1/prv1", "ws1/prv1/cls1", "ws1/prv1/cls1/app1", "ws1/prv1/cls1/app1/box1"}
	for i, doc := range result.ValidDocuments {
		if doc.FQN.String() != expectedOrder[i] {
			t.Errorf("ValidDocuments[%d] FQN = %s, want %s", i, doc.FQN, expectedOrder[i])
		}
	}
}

func TestValidator_Validate_MissingParent(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Cluster", FQN: "ws1/prv1/cls1", Object: &Cluster{}}, // Missing Provider ws1/prv1
	}

	// validator removed
	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected errors for missing parent, got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Cluster" {
		t.Errorf("Error kind = %s, want Cluster", err.Kind)
	}
	if err.FQN != "ws1/prv1/cls1" {
		t.Errorf("Error FQN = %s, want ws1/prv1/cls1", err.FQN)
	}

	// Only Workspace should be valid
	if len(result.ValidDocuments) != 1 {
		t.Errorf("Validate() returned %d valid documents, want 1", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_DuplicateFQN(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}}, // Duplicate
	}

	// validator removed
	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected errors for duplicate FQN, got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Provider" {
		t.Errorf("Error kind = %s, want Provider", err.Kind)
	}
	if err.FQN != "ws1/prv1" {
		t.Errorf("Error FQN = %s, want ws1/prv1", err.FQN)
	}

	// Only first Provider should be valid
	if len(result.ValidDocuments) != 2 {
		t.Errorf("Validate() returned %d valid documents, want 2", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_MultipleWorkspaces(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Workspace", FQN: "ws2", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "ws2/prv1", Object: &Provider{}},
	}

	// validator removed
	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	if len(result.ValidDocuments) != 4 {
		t.Errorf("Validate() returned %d valid documents, want 4", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_ComplexHierarchy(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "ws1/prv2", Object: &Provider{}},
		{Kind: "Cluster", FQN: "ws1/prv1/cls1", Object: &Cluster{}},
		{Kind: "Cluster", FQN: "ws1/prv2/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app1", Object: &App{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app2", Object: &App{}},
		{Kind: "Box", FQN: "ws1/prv1/cls1/app1/box1", Object: &Box{}},
	}

	// validator removed
	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	if len(result.ValidDocuments) != 8 {
		t.Errorf("Validate() returned %d valid documents, want 8", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_MultipleErrors(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},      // Duplicate
		{Kind: "Cluster", FQN: "ws1/prv2/cls1", Object: &Cluster{}},   // Missing parent ws1/prv2
		{Kind: "App", FQN: "ws1/prv3/cls1/app1", Object: &App{}},      // Missing parent ws1/prv3
		{Kind: "Box", FQN: "ws1/prv1/cls2/app1/box1", Object: &Box{}}, // Missing parent chain
	}

	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected multiple errors, got none")
	}

	// Should have errors for all invalid documents
	if len(result.Errors) < 4 {
		t.Errorf("Validate() returned %d errors, want at least 4", len(result.Errors))
	}

	// Should have 2 valid documents (ws1 and ws1/prv1)
	if len(result.ValidDocuments) != 2 {
		t.Errorf("Validate() returned %d valid documents, want 2", len(result.ValidDocuments))
	}
}

func TestValidator_SortByTopology(t *testing.T) {
	documents := []Document{
		{Kind: "Box", FQN: "ws1/prv1/cls1/app1/box1", Object: &Box{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app2", Object: &App{}},
		{Kind: "Workspace", FQN: "ws2", Object: &Workspace{}},
		{Kind: "Cluster", FQN: "ws1/prv1/cls1", Object: &Cluster{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "App", FQN: "ws1/prv1/cls1/app1", Object: &App{}},
	}

	sorted := sortByTopology(documents)

	// Check topological order (by kind, not by segment count)
	expectedKinds := []string{"Workspace", "Workspace", "Provider", "Cluster", "App", "App", "Box"}
	for i, doc := range sorted {
		if doc.Kind != expectedKinds[i] {
			t.Errorf("Sorted[%d] has kind %s, want %s (FQN: %s)", i, doc.Kind, expectedKinds[i], doc.FQN)
		}
	}

	// Check that within same kind, they are sorted alphabetically
	if sorted[0].FQN > sorted[1].FQN {
		t.Errorf("Within same kind, expected alphabetical order: %s should come before %s", sorted[0].FQN, sorted[1].FQN)
	}
	if sorted[4].FQN > sorted[5].FQN {
		t.Errorf("Within same kind, expected alphabetical order: %s should come before %s", sorted[4].FQN, sorted[5].FQN)
	}
}

func TestValidator_Validate_KindSegmentMismatch(t *testing.T) {
	// Provider with wrong number of segments
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1/extra", Object: &Provider{}}, // Provider should have 2 segments, not 3
	}

	// validator removed
	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for kind/segment mismatch, got nil")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Validate() expected 1 error, got %d", len(result.Errors))
	}

	if result.Errors[0].Kind != "Provider" {
		t.Errorf("Error kind = %s, want Provider", result.Errors[0].Kind)
	}
}

func TestValidator_Validate_ClusterSegmentMismatch(t *testing.T) {
	// Cluster with wrong number of segments
	documents := []Document{
		{Kind: "Workspace", FQN: "ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "ws1/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "ws1/prv1", Object: &Cluster{}}, // Cluster should have 3 segments, not 2
	}

	// validator removed
	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for kind/segment mismatch, got nil")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Validate() expected 1 error, got %d", len(result.Errors))
	}

	if result.Errors[0].Kind != "Cluster" {
		t.Errorf("Error kind = %s, want Cluster", result.Errors[0].Kind)
	}
}
