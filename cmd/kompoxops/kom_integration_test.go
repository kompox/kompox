package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitializeKOMMode(t *testing.T) {
	// Save and restore original komMode
	originalMode := komMode
	defer func() { komMode = originalMode }()

	t.Run("no KOM inputs", func(t *testing.T) {
		komMode = komModeContext{enabled: false}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("kom-path", nil, "")
		cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

		err := initializeKOMMode(cmd)
		if err != nil {
			t.Errorf("initializeKOMMode() unexpected error: %v", err)
		}
		if komMode.enabled {
			t.Errorf("KOM mode should not be enabled when no inputs provided")
		}
	})

	t.Run("kom-path does not exist", func(t *testing.T) {
		komMode = komModeContext{enabled: false}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("kom-path", nil, "")
		cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

		args := []string{"--kom-path=/nonexistent/path.yml"}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeKOMMode(cmd)
		if err == nil {
			t.Errorf("initializeKOMMode() should error on nonexistent kom-path")
		}
	})

	t.Run("load valid KOM file", func(t *testing.T) {
		komMode = komModeContext{enabled: false}

		// Create test file
		tmpDir := t.TempDir()

		// Create .git marker for base root
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.Mkdir(gitDir, 0755); err != nil {
			t.Fatal(err)
		}

		testFile := filepath.Join(tmpDir, "test.yml")
		content := `---
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: test-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/test-app
spec: {}
`
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Create dummy kompoxapp.yml to establish baseRoot context
		komAppFile := filepath.Join(tmpDir, "kompoxapp.yml")
		if err := os.WriteFile(komAppFile, []byte("# dummy\n"), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("kom-path", nil, "")
		cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

		args := []string{"--kom-path=" + testFile, "--kom-app=" + komAppFile}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeKOMMode(cmd)
		if err != nil {
			t.Errorf("initializeKOMMode() unexpected error: %v", err)
		}
		if !komMode.enabled {
			t.Errorf("KOM mode should be enabled after successful load")
		}
		if komMode.sink == nil {
			t.Errorf("KOM sink should not be nil after successful load")
		}
	})

	t.Run("infer default app name from single app", func(t *testing.T) {
		komMode = komModeContext{enabled: false}

		// Create test file with single app
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "single-app.yml")
		content := `---
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: my-single-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/my-single-app
spec: {}
`
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("kom-path", nil, "")
		cmd.PersistentFlags().String("kom-app", testFile, "")

		args := []string{"--kom-app=" + testFile}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeKOMMode(cmd)
		if err != nil {
			t.Errorf("initializeKOMMode() unexpected error: %v", err)
		}
		if !komMode.enabled {
			t.Errorf("KOM mode should be enabled after successful load")
		}
		if komMode.defaultAppID == "" {
			t.Errorf("defaultAppID should be set for single app")
		}
		// Verify the App FQN contains the app name
		if !strings.Contains(komMode.defaultAppID, "my-single-app") {
			t.Errorf("defaultAppID = %q, should contain %q", komMode.defaultAppID, "my-single-app")
		}
	})
}

// TestKOMPathRecursiveDirectoryScan tests recursive directory scanning for KOM mode
func TestKOMPathRecursiveDirectoryScan(t *testing.T) {
	komMode = komModeContext{enabled: false}
	defer func() { komMode = komModeContext{enabled: false} }()

	tmpDir := t.TempDir()

	// Create .git marker for base root
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create kompoxapp.yml
	komAppFile := filepath.Join(tmpDir, "kompoxapp.yml")
	komAppContent := `---
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
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
metadata:
  name: defaults
spec:
  komPath:
    - ./configs
`
	if err := os.WriteFile(komAppFile, []byte(komAppContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create configs directory with nested structure
	configsDir := filepath.Join(tmpDir, "configs")
	if err := os.Mkdir(configsDir, 0755); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(configsDir, "apps")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create App files in configs/apps/
	app1File := filepath.Join(subDir, "app1.yml")
	app1Content := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: app1
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/app1
spec: {}
`
	if err := os.WriteFile(app1File, []byte(app1Content), 0644); err != nil {
		t.Fatal(err)
	}

	app2File := filepath.Join(subDir, "app2.yaml")
	app2Content := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: app2
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/app2
spec: {}
`
	if err := os.WriteFile(app2File, []byte(app2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create ignored directory
	nodeModulesDir := filepath.Join(configsDir, "node_modules")
	if err := os.Mkdir(nodeModulesDir, 0755); err != nil {
		t.Fatal(err)
	}
	ignoredFile := filepath.Join(nodeModulesDir, "ignored.yml")
	if err := os.WriteFile(ignoredFile, []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize KOM mode
	cmd := &cobra.Command{}
	cmd.PersistentFlags().StringArray("kom-path", nil, "")
	cmd.PersistentFlags().String("kom-app", komAppFile, "")

	args := []string{"--kom-app=" + komAppFile}
	cmd.SetArgs(args)
	cmd.ParseFlags(args)

	err := initializeKOMMode(cmd)
	if err != nil {
		t.Fatalf("initializeKOMMode() unexpected error: %v", err)
	}

	if !komMode.enabled {
		t.Error("KOM mode should be enabled")
	}

	// Check that Apps were loaded
	if komMode.sink == nil {
		t.Fatal("sink should not be nil")
	}

	apps := komMode.sink.ListApps()
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}

	// Verify app names
	appNames := make(map[string]bool)
	for _, app := range apps {
		appNames[app.GetName()] = true
	}

	if !appNames["app1"] {
		t.Error("app1 not found")
	}
	if !appNames["app2"] {
		t.Error("app2 not found")
	}
}

// TestKOMPathParentDirectoryReference tests that parent directory references work
func TestKOMPathParentDirectoryReference(t *testing.T) {
	komMode = komModeContext{enabled: false}
	defer func() { komMode = komModeContext{enabled: false} }()

	tmpDir := t.TempDir()

	// Create .git marker at top level
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create shared configs in parent directory
	sharedDir := filepath.Join(tmpDir, "shared")
	if err := os.Mkdir(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}

	sharedFile := filepath.Join(sharedDir, "shared.yml")
	sharedContent := `---
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
	if err := os.WriteFile(sharedFile, []byte(sharedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create project subdirectory
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.Mkdir(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create kompoxapp.yml with parent reference
	komAppFile := filepath.Join(projectDir, "kompoxapp.yml")
	komAppContent := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: main-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/test-cls/app/main-app
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Defaults
metadata:
  name: defaults
spec:
  komPath:
    - ../shared
`
	if err := os.WriteFile(komAppFile, []byte(komAppContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize KOM mode
	cmd := &cobra.Command{}
	cmd.PersistentFlags().StringArray("kom-path", nil, "")
	cmd.PersistentFlags().String("kom-app", komAppFile, "")

	args := []string{"--kom-app=" + komAppFile}
	cmd.SetArgs(args)
	cmd.ParseFlags(args)

	err := initializeKOMMode(cmd)
	if err != nil {
		t.Fatalf("initializeKOMMode() unexpected error: %v", err)
	}

	if !komMode.enabled {
		t.Error("KOM mode should be enabled")
	}

	// Check that all resources were loaded
	if komMode.sink == nil {
		t.Fatal("sink should not be nil")
	}

	workspaces := komMode.sink.ListWorkspaces()
	if len(workspaces) != 1 || workspaces[0].GetName() != "test-ws" {
		t.Errorf("Expected workspace 'test-ws', got: %v", workspaces)
	}
}
