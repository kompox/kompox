package kube

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

// TestConverterVolumesSingleFileBind tests that single-file bind volumes are rejected.
func TestConverterVolumesSingleFileBind(t *testing.T) {
	ctx := context.Background()

	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(testFile, []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a test directory
	testDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Save original working directory
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origWd)

	// Change to temp directory for relative path tests
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	tests := []struct {
		name       string
		compose    string
		appVolumes []model.AppVolume
		wantErr    string
	}{
		{
			name: "single_file_bind_rejected",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - ./config.json:/etc/config.json
`,
			appVolumes: []model.AppVolume{
				{Name: "default", Size: 1024},
			},
			wantErr: "bind volume source must be a directory, not a single file",
		},
		{
			name: "directory_bind_accepted",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - ./data:/var/data
`,
			appVolumes: []model.AppVolume{
				{Name: "default", Size: 1024},
			},
			wantErr: "",
		},
		{
			name: "nonexistent_path_accepted",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - ./nonexistent:/mnt
`,
			appVolumes: []model.AppVolume{
				{Name: "default", Size: 1024},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &model.Service{Name: "testsvc"}
			prv := &model.Provider{Name: "testprv", Driver: "test"}
			cls := &model.Cluster{Name: "testcls"}
			app := &model.App{
				Name:    "testapp",
				Compose: tt.compose,
				Volumes: tt.appVolumes,
			}

			c := NewConverter(svc, prv, cls, app, "app")
			_, err := c.Convert(ctx)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
