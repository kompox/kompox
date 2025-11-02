package kompoxenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchForKompoxDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directories
	projectDir := filepath.Join(tmpDir, "project")
	subDir := filepath.Join(projectDir, "subdir")
	deepDir := filepath.Join(subDir, "deep")

	for _, dir := range []string{projectDir, subDir, deepDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("creating directory %q: %v", dir, err)
		}
	}

	// Create .kompox in project directory
	kompoxDir := filepath.Join(projectDir, KompoxCfgDirName)
	if err := os.Mkdir(kompoxDir, 0755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}

	tests := []struct {
		name      string
		startDir  string
		wantFound string
	}{
		{
			name:      "from project root",
			startDir:  projectDir,
			wantFound: projectDir,
		},
		{
			name:      "from subdirectory",
			startDir:  subDir,
			wantFound: projectDir,
		},
		{
			name:      "from deep subdirectory",
			startDir:  deepDir,
			wantFound: projectDir,
		},
		{
			name:      "not found",
			startDir:  tmpDir,
			wantFound: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := searchForKompoxDir(tt.startDir)
			if err != nil {
				t.Fatalf("searchForKompoxDir() error: %v", err)
			}
			if got != tt.wantFound {
				t.Errorf("searchForKompoxDir() = %q, want %q", got, tt.wantFound)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	kompoxDir := filepath.Join(projectDir, KompoxCfgDirName)

	if err := os.MkdirAll(kompoxDir, 0755); err != nil {
		t.Fatalf("creating directories: %v", err)
	}

	tests := []struct {
		name             string
		kompoxDir        string
		kompoxCfgDir     string
		workDir          string
		wantKompoxDir    string
		wantKompoxCfgDir string
		wantErr          bool
	}{
		{
			name:             "explicit dirs",
			kompoxDir:        projectDir,
			kompoxCfgDir:     kompoxDir,
			workDir:          tmpDir,
			wantKompoxDir:    projectDir,
			wantKompoxCfgDir: kompoxDir,
			wantErr:          false,
		},
		{
			name:             "discover from workdir",
			kompoxDir:        "",
			kompoxCfgDir:     "",
			workDir:          projectDir,
			wantKompoxDir:    projectDir,
			wantKompoxCfgDir: kompoxDir,
			wantErr:          false,
		},
		{
			name:             "explicit kompoxDir, default kompoxCfgDir",
			kompoxDir:        projectDir,
			kompoxCfgDir:     "",
			workDir:          tmpDir,
			wantKompoxDir:    projectDir,
			wantKompoxCfgDir: kompoxDir,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Resolve(tt.kompoxDir, tt.kompoxCfgDir, tt.workDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if cfg.KompoxDir != tt.wantKompoxDir {
				t.Errorf("KompoxDir = %q, want %q", cfg.KompoxDir, tt.wantKompoxDir)
			}
			if cfg.KompoxCfgDir != tt.wantKompoxCfgDir {
				t.Errorf("KompoxCfgDir = %q, want %q", cfg.KompoxCfgDir, tt.wantKompoxCfgDir)
			}
		})
	}
}

func TestConfig_ExpandVars(t *testing.T) {
	cfg := &Env{
		KompoxDir:    "/home/user/project",
		KompoxCfgDir: "/home/user/project/.kompox",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "expand KOMPOX_DIR",
			input: "$KOMPOX_DIR/kom",
			want:  "/home/user/project/kom",
		},
		{
			name:  "expand KOMPOX_CFG_DIR",
			input: "$KOMPOX_CFG_DIR/kom",
			want:  "/home/user/project/.kompox/kom",
		},
		{
			name:  "expand both",
			input: "$KOMPOX_DIR/app and $KOMPOX_CFG_DIR/kom",
			want:  "/home/user/project/app and /home/user/project/.kompox/kom",
		},
		{
			name:  "no expansion",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.ExpandVars(tt.input)
			if got != tt.want {
				t.Errorf("ExpandVars() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_IsWithinBoundary(t *testing.T) {
	cfg := &Env{
		KompoxDir:    "/home/user/project",
		KompoxCfgDir: "/home/user/project/.kompox",
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "within KOMPOX_DIR",
			path: "/home/user/project/kom/app.yml",
			want: true,
		},
		{
			name: "within KOMPOX_CFG_DIR",
			path: "/home/user/project/.kompox/kom/db.yml",
			want: true,
		},
		{
			name: "exactly KOMPOX_DIR",
			path: "/home/user/project",
			want: true,
		},
		{
			name: "exactly KOMPOX_CFG_DIR",
			path: "/home/user/project/.kompox",
			want: true,
		},
		{
			name: "outside boundary",
			path: "/home/user/other/kom/app.yml",
			want: false,
		},
		{
			name: "parent of KOMPOX_DIR",
			path: "/home/user",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.IsWithinBoundary(tt.path)
			if got != tt.want {
				t.Errorf("IsWithinBoundary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	kompoxCfgDir := filepath.Join(tmpDir, KompoxCfgDirName)
	if err := os.Mkdir(kompoxCfgDir, 0755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}

	// Test case 1: config file exists
	configPath := filepath.Join(kompoxCfgDir, ConfigFileName)
	configContent := `version: 1
store:
  type: local
komPath:
  - kom/app
  - kom/db
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg := &Env{
		KompoxDir:    tmpDir,
		KompoxCfgDir: kompoxCfgDir,
	}

	if err := cfg.loadConfigFile(); err != nil {
		t.Fatalf("loadConfigFile() error: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
	if cfg.Store.Type != "local" {
		t.Errorf("Store.Type = %q, want %q", cfg.Store.Type, "local")
	}
	if len(cfg.KOMPath) != 2 {
		t.Errorf("len(KOMPath) = %d, want 2", len(cfg.KOMPath))
	}

	// Test case 2: config file doesn't exist
	tmpDir2 := t.TempDir()
	kompoxCfgDir2 := filepath.Join(tmpDir2, KompoxCfgDirName)
	if err := os.Mkdir(kompoxCfgDir2, 0755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}

	cfg2 := &Env{
		KompoxDir:    tmpDir2,
		KompoxCfgDir: kompoxCfgDir2,
	}

	if err := cfg2.loadConfigFile(); err != nil {
		t.Fatalf("loadConfigFile() error when file missing: %v", err)
	}

	// Should have zero values
	if cfg2.Version != 0 {
		t.Errorf("Version = %d, want 0 when config missing", cfg2.Version)
	}
}
