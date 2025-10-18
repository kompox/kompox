package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetKOMPaths(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		flagVals []string
		want     []string
	}{
		{
			name:     "no paths",
			envValue: "",
			flagVals: nil,
			want:     nil,
		},
		{
			name:     "env only",
			envValue: "path1,path2",
			flagVals: nil,
			want:     []string{"path1", "path2"},
		},
		{
			name:     "env with spaces",
			envValue: " path1 , path2 ",
			flagVals: nil,
			want:     []string{"path1", "path2"},
		},
		{
			name:     "flag overrides env",
			envValue: "env1,env2",
			flagVals: []string{"flag1", "flag2"},
			want:     []string{"flag1", "flag2"},
		},
		{
			name:     "flag only",
			envValue: "",
			flagVals: []string{"flag1"},
			want:     []string{"flag1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("KOMPOX_KOM_PATH", tt.envValue)
				defer os.Unsetenv("KOMPOX_KOM_PATH")
			} else {
				os.Unsetenv("KOMPOX_KOM_PATH")
			}

			// Create command with flags
			cmd := &cobra.Command{}
			cmd.PersistentFlags().StringArray("kom-path", nil, "")

			// Parse flags to simulate command-line usage
			if tt.flagVals != nil {
				args := []string{}
				for _, v := range tt.flagVals {
					args = append(args, "--kom-path="+v)
				}
				cmd.SetArgs(args)
				cmd.ParseFlags(args)
			}

			got := getKOMPaths(cmd)

			// Compare results
			if len(got) != len(tt.want) {
				t.Errorf("getKOMPaths() length = %v, want %v (got: %v)", len(got), len(tt.want), got)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getKOMPaths()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetKOMAppPath(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		flagVal  string
		want     string
	}{
		{
			name:     "default",
			envValue: "",
			flagVal:  "",
			want:     "./kompoxapp.yml",
		},
		{
			name:     "env only",
			envValue: "custom.yml",
			flagVal:  "",
			want:     "custom.yml",
		},
		{
			name:     "flag overrides env",
			envValue: "env.yml",
			flagVal:  "flag.yml",
			want:     "flag.yml",
		},
		{
			name:     "flag only",
			envValue: "",
			flagVal:  "flag.yml",
			want:     "flag.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envValue != "" {
				os.Setenv("KOMPOX_KOM_APP", tt.envValue)
				defer os.Unsetenv("KOMPOX_KOM_APP")
			} else {
				os.Unsetenv("KOMPOX_KOM_APP")
			}

			// Create command with flag
			cmd := &cobra.Command{}
			cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

			// Parse flags if needed
			if tt.flagVal != "" {
				args := []string{"--kom-app=" + tt.flagVal}
				cmd.SetArgs(args)
				cmd.ParseFlags(args)
			}

			got := getKOMAppPath(cmd)

			if got != tt.want {
				t.Errorf("getKOMAppPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitializeKOMMode_WithDefaults(t *testing.T) {
	// Skip if test data doesn't exist
	testAppPath := "../../_tmp/tests/defaults/kompoxapp.yml"
	if _, err := os.Stat(testAppPath); os.IsNotExist(err) {
		t.Skip("Test data not found; skipping integration test")
	}

	// Save and restore current directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Change to test directory
	testDir := "../../_tmp/tests/defaults"
	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create command
	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")
	cmd.PersistentFlags().StringArray("kom-path", nil, "")

	// Clear environment
	os.Unsetenv("KOMPOX_KOM_PATH")
	os.Unsetenv("KOMPOX_KOM_APP")

	// Reset global state
	komMode = komModeContext{}

	// Initialize KOM mode
	if err := initializeKOMMode(cmd); err != nil {
		t.Fatalf("initializeKOMMode() failed: %v", err)
	}

	// Verify KOM mode is enabled
	if !komMode.enabled {
		t.Error("KOM mode should be enabled")
	}

	// Verify sink is created
	if komMode.sink == nil {
		t.Fatal("Sink should be created")
	}

	// Verify default App ID is set from Defaults
	expectedAppID := "/ws/test-ws/prv/test-prv/cls/test-cls/app/test-app"
	if komMode.defaultAppID != expectedAppID {
		t.Errorf("defaultAppID = %q, want %q", komMode.defaultAppID, expectedAppID)
	}

	// Verify default Cluster ID
	expectedClusterID := "/ws/test-ws/prv/test-prv/cls/test-cls"
	if komMode.defaultClusterID != expectedClusterID {
		t.Errorf("defaultClusterID = %q, want %q", komMode.defaultClusterID, expectedClusterID)
	}

	// Verify that kom/box.yml was loaded
	apps := komMode.sink.ListApps()
	if len(apps) == 0 {
		t.Fatal("Expected at least one App")
	}

	// Check if Box was loaded
	boxes := komMode.sink.ListBoxes()
	if len(boxes) == 0 {
		t.Error("Expected Box to be loaded from kom/box.yml")
	}
}

func TestInitializeKOMMode_LocalFSReferenceValidation(t *testing.T) {
	// This test verifies that Apps with local FS references are rejected
	// when they are not defined in kompoxapp.yml

	// Create a temporary test directory
	tmpDir := t.TempDir()

	// Create kompoxapp.yml with Defaults pointing to a kom directory
	kompoxappContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
metadata:
  name: defaults
spec:
  komPath:
    - kom
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
  annotations:
    ops.kompox.dev/id: /ws/test-ws
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-prv
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: test-cls
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls
spec:
  existing: true
`
	kompoxappPath := filepath.Join(tmpDir, "kompoxapp.yml")
	if err := os.WriteFile(kompoxappPath, []byte(kompoxappContent), 0644); err != nil {
		t.Fatalf("Failed to write kompoxapp.yml: %v", err)
	}

	// Create kom directory
	komDir := filepath.Join(tmpDir, "kom")
	if err := os.MkdirAll(komDir, 0755); err != nil {
		t.Fatalf("Failed to create kom directory: %v", err)
	}

	// Create an App with local FS reference in kom directory
	appContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: external-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/external-app
spec:
  compose: |
    services:
      web:
        image: nginx:alpine
        volumes:
          - ./data:/data
`
	appPath := filepath.Join(komDir, "app.yml")
	if err := os.WriteFile(appPath, []byte(appContent), 0644); err != nil {
		t.Fatalf("Failed to write app.yml: %v", err)
	}

	// Save and restore current directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Change to test directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create command
	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")
	cmd.PersistentFlags().StringArray("kom-path", nil, "")

	// Clear environment
	os.Unsetenv("KOMPOX_KOM_PATH")
	os.Unsetenv("KOMPOX_KOM_APP")

	// Reset global state
	komMode = komModeContext{}

	// Initialize KOM mode - should fail due to local FS reference
	err := initializeKOMMode(cmd)
	if err == nil {
		t.Fatal("Expected error for App with local FS reference outside kompoxapp.yml, but got none")
	}

	expectedErrMsg := "uses local filesystem references but is not defined in kompoxapp.yml"
	if !contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, got: %v", expectedErrMsg, err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFindBaseRoot tests base root detection
func TestFindBaseRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create structure: tmpDir/.git/
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "sub", "deep")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Base root should be tmpDir (has .git)
	baseRoot, err := findBaseRoot(subDir)
	if err != nil {
		t.Fatalf("findBaseRoot failed: %v", err)
	}

	if baseRoot != tmpDir {
		t.Errorf("Expected base root %q, got %q", tmpDir, baseRoot)
	}
}

// TestValidateAndResolveKOMPath tests path validation
func TestValidateAndResolveKOMPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create base root marker
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	validFile := filepath.Join(tmpDir, "valid.yml")
	if err := os.WriteFile(validFile, []byte("apiVersion: ops.kompox.io/v1alpha1\nkind: Cluster"), 0644); err != nil {
		t.Fatal(err)
	}

	invalidExtFile := filepath.Join(tmpDir, "invalid.txt")
	if err := os.WriteFile(invalidExtFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	validDir := filepath.Join(tmpDir, "configs")
	if err := os.Mkdir(validDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		inputPath string
		baseDir   string
		baseRoot  string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid file",
			inputPath: validFile,
			baseDir:   tmpDir,
			baseRoot:  tmpDir,
			wantErr:   false,
		},
		{
			name:      "invalid extension",
			inputPath: invalidExtFile,
			baseDir:   tmpDir,
			baseRoot:  tmpDir,
			wantErr:   true,
			errMsg:    "must have .yml or .yaml extension",
		},
		{
			name:      "directory",
			inputPath: validDir,
			baseDir:   tmpDir,
			baseRoot:  tmpDir,
			wantErr:   false,
		},
		{
			name:      "non-existent",
			inputPath: filepath.Join(tmpDir, "missing.yml"),
			baseDir:   tmpDir,
			baseRoot:  tmpDir,
			wantErr:   true,
			errMsg:    "does not exist",
		},
		{
			name:      "URL rejected",
			inputPath: "https://example.com/file.yml",
			baseDir:   tmpDir,
			baseRoot:  tmpDir,
			wantErr:   true,
			errMsg:    "URL not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := validateAndResolveKOMPath(tt.inputPath, tt.baseDir, tt.baseRoot)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got none", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resolved == "" {
					t.Error("Expected non-empty resolved path")
				}
			}
		})
	}
}

// TestScanKOMDirectory tests recursive directory scanning with limits
func TestScanKOMDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create structure
	// tmpDir/
	//   file1.yml
	//   subdir/
	//     file2.yaml
	//   .git/  (ignored)
	//     file3.yml
	//   node_modules/  (ignored)
	//     file4.yml

	file1 := filepath.Join(tmpDir, "file1.yml")
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(subDir, "file2.yaml")
	if err := os.WriteFile(file2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	file3 := filepath.Join(gitDir, "file3.yml")
	if err := os.WriteFile(file3, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	nmDir := filepath.Join(tmpDir, "node_modules")
	if err := os.Mkdir(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	file4 := filepath.Join(nmDir, "file4.yml")
	if err := os.WriteFile(file4, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	visited := make(map[string]bool)
	stats := &komScanStats{}

	files, err := scanKOMDirectory(tmpDir, tmpDir, 0, visited, stats)
	if err != nil {
		t.Fatalf("scanKOMDirectory failed: %v", err)
	}

	// Should find file1.yml and subdir/file2.yaml, but not .git/ or node_modules/
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d: %v", len(files), files)
	}

	found1, found2 := false, false
	for _, f := range files {
		if f == file1 {
			found1 = true
		}
		if f == file2 {
			found2 = true
		}
		// file3 and file4 should NOT be found
		if f == file3 || f == file4 {
			t.Errorf("Found ignored file: %s", f)
		}
	}

	if !found1 {
		t.Error("file1.yml not found")
	}
	if !found2 {
		t.Error("subdir/file2.yaml not found")
	}
}

// TestKOMPathOutsideBaseRoot tests that paths outside base root are rejected
func TestKOMPathOutsideBaseRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create base root marker
	baseRootDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(baseRootDir, 0755); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(baseRootDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file outside base root
	outsideFile := filepath.Join(tmpDir, "outside.yml")
	if err := os.WriteFile(outsideFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to validate a path outside base root
	_, err := validateAndResolveKOMPath(outsideFile, baseRootDir, baseRootDir)
	if err == nil {
		t.Error("Expected error for path outside base root, got none")
	}
	if !contains(err.Error(), "outside base root") {
		t.Errorf("Expected 'outside base root' error, got: %v", err)
	}
}
