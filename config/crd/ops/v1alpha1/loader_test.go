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
  annotations:
    ops.kompox.dev/id: /ws/ws1
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
	if doc.FQN != "/ws/ws1" {
		t.Errorf("Document FQN = %q, want %q", doc.FQN, "/ws/ws1")
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
  annotations:
    ops.kompox.dev/id: /ws/ws1
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: prv1
  annotations:
    ops.kompox.dev/id: /ws/ws1/prv/prv1
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
	if result.Documents[0].FQN != "/ws/ws1" {
		t.Errorf("Document 0 FQN = %q, want /ws/ws1", result.Documents[0].FQN)
	}

	// Check Provider
	if result.Documents[1].Kind != "Provider" {
		t.Errorf("Document 1 kind = %q, want Provider", result.Documents[1].Kind)
	}
	if result.Documents[1].FQN != "/ws/ws1/prv/prv1" {
		t.Errorf("Document 1 FQN = %q, want /ws/ws1/prv/prv1", result.Documents[1].FQN)
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
  annotations:
    ops.kompox.dev/id: /ws/ws1
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
    ops.kompox.dev/id: /ws/ws1/prv/prv1
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
    ops.kompox.dev/id: /ws/ws1/prv/prv1/cls/cls1
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
    ops.kompox.dev/id: /ws/ws1
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
  annotations:
    ops.kompox.dev/id: /ws/ws1
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

// TestLoader_TypeConversion tests that various value types (numbers, booleans, strings)
// in settings and resources maps are automatically converted to strings, matching the
// behavior of kompoxopscfg.
func TestLoader_TypeConversion(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-types.yml")

	yamlContent := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
  annotations:
    ops.kompox.dev/id: /ws/test-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-provider
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-provider
spec:
  driver: aks
  settings:
    STRING_VALUE: "hello"
    INT_VALUE: 64
    FLOAT_VALUE: 3.14
    BOOL_TRUE: true
    BOOL_FALSE: false
    ZERO: 0
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-provider/cls/test-cluster
spec:
  existing: false
  settings:
    DISK_SIZE_GB: 128
    VM_COUNT: 3
    ENABLE_FEATURE: true
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: test-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-provider/cls/test-cluster/app/test-app
spec:
  compose: "services: {}"
  resources:
    CPU_LIMIT: 2
    MEMORY_MB: 1024
  settings:
    PORT: 8080
    WORKERS: 4
    DEBUG: false
`

	if err := os.WriteFile(testFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(result.Errors) > 0 {
		t.Fatalf("Load() returned errors: %v", result.Errors)
	}

	// Extract objects
	var provider *Provider
	var cluster *Cluster
	var app *App
	for _, doc := range result.Documents {
		switch doc.Kind {
		case "Provider":
			provider = doc.Object.(*Provider)
		case "Cluster":
			cluster = doc.Object.(*Cluster)
		case "App":
			app = doc.Object.(*App)
		}
	}

	if provider == nil {
		t.Fatal("Provider document not found")
	}
	if cluster == nil {
		t.Fatal("Cluster document not found")
	}
	if app == nil {
		t.Fatal("App document not found")
	}

	// Test all settings and resources
	tests := []struct {
		name  string
		got   string
		want  string
		found bool
	}{
		// Provider settings
		{"Provider.Settings.STRING_VALUE", provider.Spec.Settings["STRING_VALUE"], "hello", provider.Spec.Settings["STRING_VALUE"] != ""},
		{"Provider.Settings.INT_VALUE", provider.Spec.Settings["INT_VALUE"], "64", provider.Spec.Settings["INT_VALUE"] != ""},
		{"Provider.Settings.FLOAT_VALUE", provider.Spec.Settings["FLOAT_VALUE"], "3.14", provider.Spec.Settings["FLOAT_VALUE"] != ""},
		{"Provider.Settings.BOOL_TRUE", provider.Spec.Settings["BOOL_TRUE"], "true", provider.Spec.Settings["BOOL_TRUE"] != ""},
		{"Provider.Settings.BOOL_FALSE", provider.Spec.Settings["BOOL_FALSE"], "false", provider.Spec.Settings["BOOL_FALSE"] != ""},
		{"Provider.Settings.ZERO", provider.Spec.Settings["ZERO"], "0", provider.Spec.Settings["ZERO"] != ""},
		// Cluster settings
		{"Cluster.Settings.DISK_SIZE_GB", cluster.Spec.Settings["DISK_SIZE_GB"], "128", cluster.Spec.Settings["DISK_SIZE_GB"] != ""},
		{"Cluster.Settings.VM_COUNT", cluster.Spec.Settings["VM_COUNT"], "3", cluster.Spec.Settings["VM_COUNT"] != ""},
		{"Cluster.Settings.ENABLE_FEATURE", cluster.Spec.Settings["ENABLE_FEATURE"], "true", cluster.Spec.Settings["ENABLE_FEATURE"] != ""},
		// App resources
		{"App.Resources.CPU_LIMIT", app.Spec.Resources["CPU_LIMIT"], "2", app.Spec.Resources["CPU_LIMIT"] != ""},
		{"App.Resources.MEMORY_MB", app.Spec.Resources["MEMORY_MB"], "1024", app.Spec.Resources["MEMORY_MB"] != ""},
		// App settings
		{"App.Settings.PORT", app.Spec.Settings["PORT"], "8080", app.Spec.Settings["PORT"] != ""},
		{"App.Settings.WORKERS", app.Spec.Settings["WORKERS"], "4", app.Spec.Settings["WORKERS"] != ""},
		{"App.Settings.DEBUG", app.Spec.Settings["DEBUG"], "false", app.Spec.Settings["DEBUG"] != ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.found {
				t.Errorf("Key not found")
				return
			}
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
