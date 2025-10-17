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

		cmd := &cobra.Command{}
		cmd.PersistentFlags().StringArray("kom-path", nil, "")
		cmd.PersistentFlags().String("kom-app", "./kompoxapp.yml", "")

		args := []string{"--kom-path=" + testFile}
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
