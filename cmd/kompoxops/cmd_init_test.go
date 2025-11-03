package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kompox/kompox/config/kompoxenv"
	"gopkg.in/yaml.v3"
)

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name          string
		existingFiles map[string]string // path -> content
		forceFlag     bool
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name:          "new_directory",
			existingFiles: nil,
			forceFlag:     false,
			wantErr:       false,
		},
		{
			name: "existing_config_no_force",
			existingFiles: map[string]string{
				".kompox/config.yml": "version: 1\n",
			},
			forceFlag:  false,
			wantErr:    true,
			wantErrMsg: "already exists",
		},
		{
			name: "existing_config_with_force",
			existingFiles: map[string]string{
				".kompox/config.yml": "version: 1\n",
			},
			forceFlag: true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create existing files
			for relPath, content := range tt.existingFiles {
				fullPath := filepath.Join(tmpDir, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("creating parent directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("creating existing file: %v", err)
				}
			}

			// Change to temp directory
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("getting working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(oldWd); err != nil {
					t.Errorf("restoring working directory: %v", err)
				}
			}()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("changing to temp directory: %v", err)
			}

			// Set up command
			cmd := newCmdInit()
			if tt.forceFlag {
				cmd.Flags().Set("force", "true")
			}

			// Run command
			err = runInit(cmd, nil, tt.forceFlag)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErrMsg)
				} else if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify .kompox/ directory exists
			kompoxCfgDir := filepath.Join(tmpDir, kompoxenv.KompoxCfgDirName)
			if _, err := os.Stat(kompoxCfgDir); os.IsNotExist(err) {
				t.Errorf(".kompox/ directory not created")
			}

			// Verify config.yml exists and has correct structure
			configPath := filepath.Join(kompoxCfgDir, kompoxenv.ConfigFileName)
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("reading config.yml: %v", err)
			}

			var config map[string]interface{}
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Fatalf("parsing config.yml: %v", err)
			}

			// Check version
			if version, ok := config["version"].(int); !ok || version != 1 {
				t.Errorf("expected version=1, got %v", config["version"])
			}

			// Check store
			if store, ok := config["store"].(map[string]interface{}); !ok {
				t.Errorf("expected store to be map, got %T", config["store"])
			} else if storeType, ok := store["type"].(string); !ok || storeType != "local" {
				t.Errorf("expected store.type=local, got %v", store["type"])
			}

			// Check komPath
			if komPath, ok := config["komPath"].([]interface{}); !ok {
				t.Errorf("expected komPath to be array, got %T", config["komPath"])
			} else if len(komPath) != 1 || komPath[0] != "kom" {
				t.Errorf("expected komPath=[\"kom\"], got %v", komPath)
			}

			// Verify .kompox/kom/ directory exists
			komDir := filepath.Join(kompoxCfgDir, "kom")
			if _, err := os.Stat(komDir); os.IsNotExist(err) {
				t.Errorf(".kompox/kom/ directory not created")
			}
		})
	}
}

func TestInitCommand_WithCFlag(t *testing.T) {
	// Save and restore working directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	}()

	// Create temporary directory
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "new", "nested", "project")

	// Set up command hierarchy (root -> init)
	rootCmd := newRootCmd()
	initCmd := newCmdInit()
	rootCmd.AddCommand(initCmd)

	// Set -C flag on root command
	rootCmd.PersistentFlags().Set("chdir", targetDir)

	// Run command
	err = runInit(initCmd, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify target directory was created
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		t.Errorf("target directory not created: %s", targetDir)
	}

	// Verify .kompox/ directory exists in target
	kompoxCfgDir := filepath.Join(targetDir, kompoxenv.KompoxCfgDirName)
	if _, err := os.Stat(kompoxCfgDir); os.IsNotExist(err) {
		t.Errorf(".kompox/ directory not created in target: %s", kompoxCfgDir)
	}

	// Verify config.yml exists
	configPath := filepath.Join(kompoxCfgDir, kompoxenv.ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config.yml not created: %s", configPath)
	}
}
