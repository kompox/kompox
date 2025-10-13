package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitializeCRDMode(t *testing.T) {
	// Save and restore original crdMode
	originalMode := crdMode
	defer func() { crdMode = originalMode }()

	t.Run("no CRD inputs", func(t *testing.T) {
		crdMode = crdModeContext{enabled: false}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("crd-path", nil, "")
		cmd.PersistentFlags().String("crd-app", "./kompoxapp.yml", "")

		err := initializeCRDMode(cmd)
		if err != nil {
			t.Errorf("initializeCRDMode() unexpected error: %v", err)
		}
		if crdMode.enabled {
			t.Errorf("CRD mode should not be enabled when no inputs provided")
		}
	})

	t.Run("crd-path does not exist", func(t *testing.T) {
		crdMode = crdModeContext{enabled: false}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("crd-path", nil, "")
		cmd.PersistentFlags().String("crd-app", "./kompoxapp.yml", "")

		args := []string{"--crd-path=/nonexistent/path.yml"}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeCRDMode(cmd)
		if err == nil {
			t.Errorf("initializeCRDMode() should error on nonexistent crd-path")
		}
	})

	t.Run("load valid CRD file", func(t *testing.T) {
		crdMode = crdModeContext{enabled: false}

		// Create test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yml")
		content := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-prv
  annotations:
    ops.kompox.dev/path: test-ws
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: test-cls
  annotations:
    ops.kompox.dev/path: test-ws/test-prv
spec:
  existing: true
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: test-app
  annotations:
    ops.kompox.dev/path: test-ws/test-prv/test-cls
spec: {}
`
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("crd-path", nil, "")
		cmd.PersistentFlags().String("crd-app", "./kompoxapp.yml", "")

		args := []string{"--crd-path=" + testFile}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeCRDMode(cmd)
		if err != nil {
			t.Errorf("initializeCRDMode() unexpected error: %v", err)
		}
		if !crdMode.enabled {
			t.Errorf("CRD mode should be enabled after successful load")
		}
		if crdMode.sink == nil {
			t.Errorf("CRD sink should not be nil after successful load")
		}
	})

	t.Run("infer default app name from single app", func(t *testing.T) {
		crdMode = crdModeContext{enabled: false}

		// Create test file with single app
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "single-app.yml")
		content := `---
apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
spec: {}
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-prv
  annotations:
    ops.kompox.dev/path: test-ws
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: test-cls
  annotations:
    ops.kompox.dev/path: test-ws/test-prv
spec:
  existing: true
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: my-single-app
  annotations:
    ops.kompox.dev/path: test-ws/test-prv/test-cls
spec: {}
`
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("crd-path", nil, "")
		cmd.PersistentFlags().String("crd-app", testFile, "")

		args := []string{"--crd-app=" + testFile}
		cmd.SetArgs(args)
		cmd.ParseFlags(args)

		err := initializeCRDMode(cmd)
		if err != nil {
			t.Errorf("initializeCRDMode() unexpected error: %v", err)
		}
		if !crdMode.enabled {
			t.Errorf("CRD mode should be enabled after successful load")
		}
		if crdMode.defaultAppID == "" {
			t.Errorf("defaultAppID should be set for single app")
		}
		// Verify the App FQN contains the app name
		if !strings.Contains(crdMode.defaultAppID, "my-single-app") {
			t.Errorf("defaultAppID = %q, should contain %q", crdMode.defaultAppID, "my-single-app")
		}
	})
}
