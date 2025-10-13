package v1alpha1

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to load documents from directory
func loadDocuments(t *testing.T, dir string) []Document {
	t.Helper()
	loader := NewLoader()
	loadResult, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loadResult.Errors) > 0 {
		t.Fatalf("Load() errors = %v", loadResult.Errors)
	}
	return loadResult.Documents
}

func TestNewSink_HappyPath(t *testing.T) {
	// Create test files
	tmpDir := t.TempDir()
	wsFile := filepath.Join(tmpDir, "workspace.yaml")
	wsContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`
	if err := os.WriteFile(wsFile, []byte(wsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load documents
	loader := NewLoader()
	loadResult, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Create sink
	sink, err := NewSink(loadResult.Documents)
	if err != nil {
		t.Fatalf("NewSink() error = %v", err)
	}

	// Verify
	if sink.Count() != 1 {
		t.Errorf("Sink count = %d, want 1", sink.Count())
	}

	ws, ok := sink.GetWorkspace("ws1")
	if !ok {
		t.Fatal("GetWorkspace() returned false")
	}
	if ws.Name != "ws1" {
		t.Errorf("Workspace name = %s, want ws1", ws.Name)
	}
}

func TestNewSink_MultipleDocuments(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files
	files := map[string]string{
		"workspace.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`,
		"provider.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`,
		"cluster.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/path: ws1/prv1
spec:
  existing: false
`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Load sink
	docs := loadDocuments(t, tmpDir)
	sink, err := NewSink(docs)
	if err != nil {
		t.Fatalf("LoadSink() error = %v", err)
	}

	// Verify counts
	if sink.Count() != 3 {
		t.Errorf("Sink count = %d, want 3", sink.Count())
	}

	// Verify workspace
	ws, ok := sink.GetWorkspace("ws1")
	if !ok {
		t.Fatal("GetWorkspace() returned false")
	}
	if ws.Name != "ws1" {
		t.Errorf("Workspace name = %s, want ws1", ws.Name)
	}

	// Verify provider
	prv, ok := sink.GetProvider("ws1/prv1")
	if !ok {
		t.Fatal("GetProvider() returned false")
	}
	if prv.Name != "prv1" {
		t.Errorf("Provider name = %s, want prv1", prv.Name)
	}

	// Verify cluster
	cls, ok := sink.GetCluster("ws1/prv1/cls1")
	if !ok {
		t.Fatal("GetCluster() returned false")
	}
	if cls.Name != "cls1" {
		t.Errorf("Cluster name = %s, want cls1", cls.Name)
	}
}

func TestNewSink_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create provider without workspace (missing parent)
	prvFile := filepath.Join(tmpDir, "provider.yaml")
	prvContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`
	if err := os.WriteFile(prvFile, []byte(prvContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load sink (should fail validation)
	docs := loadDocuments(t, tmpDir)
	_, err := NewSink(docs)
	if err == nil {
		t.Fatal("LoadSink() expected error for missing parent, got nil")
	}
}

func TestNewSink_DuplicateFQN(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two workspaces with the same name
	content := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`
	wsFile := filepath.Join(tmpDir, "workspaces.yaml")
	if err := os.WriteFile(wsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load sink (should fail validation)
	docs := loadDocuments(t, tmpDir)
	_, err := NewSink(docs)
	if err == nil {
		t.Fatal("LoadSink() expected error for duplicate FQN, got nil")
	}
}

func TestSink_ListMethods(t *testing.T) {
	tmpDir := t.TempDir()

	// Create comprehensive test files
	files := map[string]string{
		"resources.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws2
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/path: ws1/prv1
spec:
  existing: false
`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	// Load sink
	docs := loadDocuments(t, tmpDir)
	sink, err := NewSink(docs)
	if err != nil {
		t.Fatalf("LoadSink() error = %v", err)
	}

	// Test ListWorkspaces
	workspaces := sink.ListWorkspaces()
	if len(workspaces) != 2 {
		t.Errorf("ListWorkspaces() count = %d, want 2", len(workspaces))
	}

	// Test ListProviders
	providers := sink.ListProviders()
	if len(providers) != 1 {
		t.Errorf("ListProviders() count = %d, want 1", len(providers))
	}

	// Test ListClusters
	clusters := sink.ListClusters()
	if len(clusters) != 1 {
		t.Errorf("ListClusters() count = %d, want 1", len(clusters))
	}

	// Test ListApps
	apps := sink.ListApps()
	if len(apps) != 0 {
		t.Errorf("ListApps() count = %d, want 0", len(apps))
	}

	// Test ListBoxes
	boxes := sink.ListBoxes()
	if len(boxes) != 0 {
		t.Errorf("ListBoxes() count = %d, want 0", len(boxes))
	}
}

func TestSink_GetMethods(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	content := `apiVersion: ops.kompox.dev/v1alpha1
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
	wsFile := filepath.Join(tmpDir, "resources.yaml")
	if err := os.WriteFile(wsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load sink
	docs := loadDocuments(t, tmpDir)
	sink, err := NewSink(docs)
	if err != nil {
		t.Fatalf("LoadSink() error = %v", err)
	}

	// Test GetWorkspace (exists)
	ws, ok := sink.GetWorkspace("ws1")
	if !ok {
		t.Error("GetWorkspace(ws1) returned false, want true")
	}
	if ws == nil || ws.Name != "ws1" {
		t.Errorf("GetWorkspace(ws1) name = %v, want ws1", ws)
	}

	// Test GetWorkspace (not exists)
	_, ok = sink.GetWorkspace("nonexistent")
	if ok {
		t.Error("GetWorkspace(nonexistent) returned true, want false")
	}

	// Test GetProvider (exists)
	prv, ok := sink.GetProvider("ws1/prv1")
	if !ok {
		t.Error("GetProvider(ws1/prv1) returned false, want true")
	}
	if prv == nil || prv.Name != "prv1" {
		t.Errorf("GetProvider(ws1/prv1) name = %v, want prv1", prv)
	}

	// Test GetProvider (not exists)
	_, ok = sink.GetProvider("ws1/nonexistent")
	if ok {
		t.Error("GetProvider(ws1/nonexistent) returned true, want false")
	}
}

func TestSink_Immutability(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	wsFile := filepath.Join(tmpDir, "workspace.yaml")
	wsContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`
	if err := os.WriteFile(wsFile, []byte(wsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load sink
	docs := loadDocuments(t, tmpDir)
	sink, err := NewSink(docs)
	if err != nil {
		t.Fatalf("LoadSink() error = %v", err)
	}

	// Verify initial state
	if sink.Count() != 1 {
		t.Errorf("Initial count = %d, want 1", sink.Count())
	}

	// Try to get workspace and modify it (should not affect sink)
	ws, ok := sink.GetWorkspace("ws1")
	if !ok {
		t.Fatal("GetWorkspace() returned false")
	}

	originalName := ws.Name
	ws.Name = "modified"

	// Verify sink is unchanged
	ws2, _ := sink.GetWorkspace("ws1")
	if ws2.Name != originalName {
		t.Errorf("Sink was mutated: name = %s, want %s", ws2.Name, originalName)
	}
}

func TestNewSink_PathTracking(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	wsFile := filepath.Join(tmpDir, "workspace.yaml")
	wsContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`
	if err := os.WriteFile(wsFile, []byte(wsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load documents
	loader := NewLoader()
	loadResult, err := loader.Load(wsFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify Path and Index are set
	if len(loadResult.Documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(loadResult.Documents))
	}

	doc := loadResult.Documents[0]
	if doc.Path != wsFile {
		t.Errorf("Document Path = %s, want %s", doc.Path, wsFile)
	}
	if doc.Index != 1 {
		t.Errorf("Document Index = %d, want 1", doc.Index)
	}
}

func TestNewSink_MultipleSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := map[string]string{
		"workspace.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ws1
spec: {}
`,
		"provider.yaml": `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`,
	}

	var allDocuments []Document
	loader := NewLoader()

	for filename, content := range files {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", filename, err)
		}

		result, err := loader.Load(path)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", filename, err)
		}

		// Verify each document has correct Path and Index
		for _, doc := range result.Documents {
			if doc.Path != path {
				t.Errorf("Document from %s has Path = %s", filename, doc.Path)
			}
			if doc.Index != 1 {
				t.Errorf("Document from %s has Index = %d, want 1", filename, doc.Index)
			}
		}

		allDocuments = append(allDocuments, result.Documents...)
	}

	// Create sink with all documents
	sink, err := NewSink(allDocuments)
	if err != nil {
		t.Fatalf("NewSink() error = %v", err)
	}

	if sink.Count() != 2 {
		t.Errorf("Sink count = %d, want 2", sink.Count())
	}
}

func TestNewSink_ValidationErrorWithPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create provider without workspace (missing parent)
	prvFile := filepath.Join(tmpDir, "provider.yaml")
	prvContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/path: ws1
spec:
  driver: aks
`
	if err := os.WriteFile(prvFile, []byte(prvContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load documents
	docs := loadDocuments(t, prvFile)

	// Try to create sink (should fail with path in error)
	_, err := NewSink(docs)
	if err == nil {
		t.Fatal("NewSink() expected error for missing parent, got nil")
	}

	// Verify error message contains path and document index
	errMsg := err.Error()
	if !contains(errMsg, prvFile) {
		t.Errorf("Error message should contain path %s, got: %s", prvFile, errMsg)
	}
	if !contains(errMsg, "document 1") {
		t.Errorf("Error message should contain document index, got: %s", errMsg)
	}
}

func TestNewSink_MultiDocumentIndex(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a multi-document YAML file
	multiDocFile := filepath.Join(tmpDir, "multi.yaml")
	multiDocContent := `apiVersion: ops.kompox.dev/v1alpha1
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cls1
  annotations:
    ops.kompox.dev/path: ws1/prv1
spec: {}
`
	if err := os.WriteFile(multiDocFile, []byte(multiDocContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load documents
	loader := NewLoader()
	loadResult, err := loader.Load(multiDocFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify we loaded 3 documents
	if len(loadResult.Documents) != 3 {
		t.Fatalf("Expected 3 documents, got %d", len(loadResult.Documents))
	}

	// Verify each document has correct Path and Index
	expectedKinds := []string{"Workspace", "Provider", "Cluster"}
	for i, doc := range loadResult.Documents {
		if doc.Path != multiDocFile {
			t.Errorf("Document %d Path = %s, want %s", i, doc.Path, multiDocFile)
		}
		expectedIndex := i + 1 // 1-based indexing
		if doc.Index != expectedIndex {
			t.Errorf("Document %d Index = %d, want %d", i, doc.Index, expectedIndex)
		}
		if doc.Kind != expectedKinds[i] {
			t.Errorf("Document %d Kind = %s, want %s", i, doc.Kind, expectedKinds[i])
		}
	}

	// Create sink and verify it works
	sink, err := NewSink(loadResult.Documents)
	if err != nil {
		t.Fatalf("NewSink() error = %v", err)
	}

	if sink.Count() != 3 {
		t.Errorf("Sink count = %d, want 3", sink.Count())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInner(s, substr))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
