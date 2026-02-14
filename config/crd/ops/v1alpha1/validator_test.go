package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidator_Validate_HappyPath(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "box1"},
		}},
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
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "box1"},
		}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
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
	expectedOrder := []string{"/ws/ws1", "/ws/ws1/prv/prv1", "/ws/ws1/prv/prv1/cls/cls1", "/ws/ws1/prv/prv1/cls/cls1/app/app1", "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1"}
	for i, doc := range result.ValidDocuments {
		if doc.FQN.String() != expectedOrder[i] {
			t.Errorf("ValidDocuments[%d] FQN = %s, want %s", i, doc.FQN, expectedOrder[i])
		}
	}
}

func TestValidator_Validate_MissingParent(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}}, // Missing Provider ws1/prv1
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
	if err.FQN != "/ws/ws1/prv/prv1/cls/cls1" {
		t.Errorf("Error FQN = %s, want /ws/ws1/prv/prv1/cls/cls1", err.FQN)
	}

	// Only Workspace should be valid
	if len(result.ValidDocuments) != 1 {
		t.Errorf("Validate() returned %d valid documents, want 1", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_DuplicateFQN(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}}, // Duplicate
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
	if err.FQN != "/ws/ws1/prv/prv1" {
		t.Errorf("Error FQN = %s, want /ws/ws1/prv/prv1", err.FQN)
	}

	// Only first Provider should be valid
	if len(result.ValidDocuments) != 2 {
		t.Errorf("Validate() returned %d valid documents, want 2", len(result.ValidDocuments))
	}
}

func TestValidator_Validate_MultipleWorkspaces(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Workspace", FQN: "/ws/ws2", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "/ws/ws2/prv/prv1", Object: &Provider{}},
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
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv2", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv2/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app2", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "box1"},
		}},
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
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},      // Duplicate
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv2/cls/cls1", Object: &Cluster{}},   // Missing parent ws1/prv2
		{Kind: "App", FQN: "ws1/prv3/cls1/app1", Object: &App{}},      // Missing parent ws1/prv3
		{Kind: "Box", FQN: "ws1/prv1/cls2/app1/box1", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "box1"},
		}}, // Missing parent chain
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
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/box1", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "box1"},
		}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app2", Object: &App{}},
		{Kind: "Workspace", FQN: "/ws/ws2", Object: &Workspace{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
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
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1/cls/extra", Object: &Provider{}}, // Provider should have 2 kind/name pairs, not 3
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
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1", Object: &Cluster{}}, // Cluster should have 3 kind/name pairs, not 2
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

func TestValidator_Box_ValidComposeBox(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/web", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "web"},
			Spec:       BoxSpec{},
		}},
	}

	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	if len(result.ValidDocuments) != 5 {
		t.Errorf("Validate() returned %d valid documents, want 5", len(result.ValidDocuments))
	}
}

func TestValidator_Box_ValidStandaloneBox(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/runner", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "runner"},
			Spec: BoxSpec{
				Image:   "ubuntu:22.04",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "sleep infinity"},
			},
		}},
	}

	result := Validate(documents)

	if result.HasErrors() {
		t.Errorf("Validate() returned errors: %v", result.Errors)
	}

	if len(result.ValidDocuments) != 5 {
		t.Errorf("Validate() returned %d valid documents, want 5", len(result.ValidDocuments))
	}
}

func TestValidator_Box_InvalidReservedName(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/app", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "app"},
			Spec:       BoxSpec{},
		}},
	}

	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for reserved name 'app', got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Box" {
		t.Errorf("Error kind = %s, want Box", err.Kind)
	}
}

func TestValidator_Box_InvalidComponentMismatch(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/web", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "web"},
			Spec:       BoxSpec{Component: "different"},
		}},
	}

	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for component mismatch, got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Box" {
		t.Errorf("Error kind = %s, want Box", err.Kind)
	}
}

func TestValidator_Box_ComposeBoxWithCommand(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/web", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "web"},
			Spec: BoxSpec{
				// No image, should be Compose Box
				Command: []string{"/bin/bash"}, // But has command - invalid
			},
		}},
	}

	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for Compose Box with command, got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Box" {
		t.Errorf("Error kind = %s, want Box", err.Kind)
	}
}

func TestValidator_Box_StandaloneBoxWithIngress(t *testing.T) {
	documents := []Document{
		{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
		{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
		{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
		{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
		{Kind: "Box", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1/box/runner", Object: &Box{
			ObjectMeta: metav1.ObjectMeta{Name: "runner"},
			Spec: BoxSpec{
				Image:   "ubuntu:22.04",
				Ingress: &BoxIngressSpec{}, // Invalid: ingress is reserved
			},
		}},
	}

	result := Validate(documents)

	if !result.HasErrors() {
		t.Error("Validate() expected error for Standalone Box with ingress, got none")
	}

	if len(result.Errors) != 1 {
		t.Fatalf("Validate() returned %d errors, want 1", len(result.Errors))
	}

	err := result.Errors[0]
	if err.Kind != "Box" {
		t.Errorf("Error kind = %s, want Box", err.Kind)
	}
}

func TestValidator_Box_InvalidDNS1123Label(t *testing.T) {
	testCases := []struct {
		name     string
		boxName  string
		wantErr  bool
	}{
		{
			name:    "uppercase letters",
			boxName: "WebServer",
			wantErr: true,
		},
		{
			name:    "leading hyphen",
			boxName: "-web",
			wantErr: true,
		},
		{
			name:    "trailing hyphen",
			boxName: "web-",
			wantErr: true,
		},
		{
			name:    "too long",
			boxName: "this-is-a-very-long-name-that-exceeds-sixty-three-characters-limit",
			wantErr: true,
		},
		{
			name:    "underscore",
			boxName: "web_server",
			wantErr: true,
		},
		{
			name:    "valid with hyphens",
			boxName: "web-server-01",
			wantErr: false,
		},
		{
			name:    "valid numeric",
			boxName: "123",
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			documents := []Document{
				{Kind: "Workspace", FQN: "/ws/ws1", Object: &Workspace{}},
				{Kind: "Provider", FQN: "/ws/ws1/prv/prv1", Object: &Provider{}},
				{Kind: "Cluster", FQN: "/ws/ws1/prv/prv1/cls/cls1", Object: &Cluster{}},
				{Kind: "App", FQN: "/ws/ws1/prv/prv1/cls/cls1/app/app1", Object: &App{}},
				{Kind: "Box", FQN: FQN("/ws/ws1/prv/prv1/cls/cls1/app/app1/box/" + tc.boxName), Object: &Box{
					ObjectMeta: metav1.ObjectMeta{Name: tc.boxName},
					Spec:       BoxSpec{},
				}},
			}

			result := Validate(documents)

			if tc.wantErr && !result.HasErrors() {
				t.Errorf("Validate() expected error for name %q, got none", tc.boxName)
			}

			if !tc.wantErr && result.HasErrors() {
				t.Errorf("Validate() expected no error for name %q, got: %v", tc.boxName, result.Errors)
			}
		})
	}
}
