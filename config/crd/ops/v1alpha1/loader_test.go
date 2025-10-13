package v1alpha1

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadFile_SingleDocument(t *testing.T) {
	// Create a temporary YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec:
  settings:
    foo: bar
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Load() returned %d errors: %v", len(result.Errors), result.Errors)
	}

	if len(result.Documents) != 1 {
		t.Fatalf("Load() returned %d documents, want 1", len(result.Documents))
	}

	doc := result.Documents[0]
	if doc.Kind != "Workspace" {
		t.Errorf("Document kind = %q, want %q", doc.Kind, "Workspace")
	}
	if doc.FQN != "ws1" {
		t.Errorf("Document FQN = %q, want %q", doc.FQN, "ws1")
	}

	ws, ok := doc.Object.(*Workspace)
	if !ok {
		t.Fatalf("Document object is not *Workspace, got %T", doc.Object)
	}
	if ws.Name != "ws1" {
		t.Errorf("Workspace name = %q, want %q", ws.Name, "ws1")
	}
}

func TestLoader_LoadFile_MultiDocument(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "multi.yaml")

	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Load() returned errors: %v", result.Errors)
	}

	if len(result.Documents) != 2 {
		t.Fatalf("Load() returned %d documents, want 2", len(result.Documents))
	}

	// Check Workspace
	if result.Documents[0].Kind != "Workspace" {
		t.Errorf("Document 0 kind = %q, want Workspace", result.Documents[0].Kind)
	}
	if result.Documents[0].FQN != "ws1" {
		t.Errorf("Document 0 FQN = %q, want ws1", result.Documents[0].FQN)
	}

	// Check Provider
	if result.Documents[1].Kind != "Provider" {
		t.Errorf("Document 1 kind = %q, want Provider", result.Documents[1].Kind)
	}
	if result.Documents[1].FQN != "ws1/prv1" {
		t.Errorf("Document 1 FQN = %q, want ws1/prv1", result.Documents[1].FQN)
	}
}

func TestLoader_LoadDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files
	ws1 := filepath.Join(tmpDir, "workspace.yml")
	wsContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`
	if err := os.WriteFile(ws1, []byte(wsContent), 0644); err != nil {
		t.Fatalf("Failed to write workspace file: %v", err)
	}

	prv1 := filepath.Join(tmpDir, "provider.yaml")
	prvContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`
	if err := os.WriteFile(prv1, []byte(prvContent), 0644); err != nil {
		t.Fatalf("Failed to write provider file: %v", err)
	}

	// Create a subdirectory with another file
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	cls1 := filepath.Join(subDir, "cluster.yaml")
	clsContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/path: ws1/prv1
spec:
  existing: false
`
	if err := os.WriteFile(cls1, []byte(clsContent), 0644); err != nil {
		t.Fatalf("Failed to write cluster file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Load() returned errors: %v", result.Errors)
	}

	if len(result.Documents) != 3 {
		t.Fatalf("Load() returned %d documents, want 3", len(result.Documents))
	}

	// Check that all kinds are present
	kinds := make(map[string]bool)
	for _, doc := range result.Documents {
		kinds[doc.Kind] = true
	}

	expectedKinds := []string{"Workspace", "Provider", "Cluster"}
	for _, kind := range expectedKinds {
		if !kinds[kind] {
			t.Errorf("Missing kind: %s", kind)
		}
	}
}

func TestLoader_LoadFile_InvalidPath(t *testing.T) {
	loader := NewLoader()
	result, err := loader.Load("/nonexistent/path/file.yaml")
	if err == nil {
		t.Error("Load() expected error for nonexistent file, got nil")
	}
	if result != nil {
		t.Errorf("Load() expected nil result for nonexistent file, got %v", result)
	}
}

func TestLoader_LoadFile_MissingAnnotation(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")

	// Provider without parent path annotation
	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
spec:
  driver: aks
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have an error about missing annotation
	if len(result.Errors) == 0 {
		t.Error("Load() expected errors for missing annotation, got none")
	}

	// Should have no valid documents
	if len(result.Documents) != 0 {
		t.Errorf("Load() returned %d documents, want 0", len(result.Documents))
	}
}

func TestLoader_LoadFile_InvalidSegments(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")

	// Cluster with Provider path (should be Cluster path with 3 segments)
	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/path: ws1
spec: {}
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have an error about segment count
	if len(result.Errors) == 0 {
		t.Error("Load() expected errors for invalid segment count, got none")
	}

	// Should have no valid documents
	if len(result.Documents) != 0 {
		t.Errorf("Load() returned %d documents, want 0", len(result.Documents))
	}
}

func TestLoader_LoadFile_InvalidDNS1123Label(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")

	// Workspace with uppercase name (invalid DNS-1123 label)
	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: WS1
spec: {}
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should have an error about DNS-1123 validation
	if len(result.Errors) == 0 {
		t.Error("Load() expected errors for invalid DNS-1123 label, got none")
	}

	// Should have no valid documents
	if len(result.Documents) != 0 {
		t.Errorf("Load() returned %d documents, want 0", len(result.Documents))
	}
}

func TestLoader_LoadFile_SkipNonKompoxDocuments(t *testing.T) {
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "mixed.yaml")

	yamlContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec: {}
`

	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Load() returned errors: %v", result.Errors)
	}

	// Should only have the Kompox document
	if len(result.Documents) != 1 {
		t.Fatalf("Load() returned %d documents, want 1", len(result.Documents))
	}

	if result.Documents[0].Kind != "Workspace" {
		t.Errorf("Document kind = %q, want Workspace", result.Documents[0].Kind)
	}
}
