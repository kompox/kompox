package kompoxenv

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
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
	kompoxDir := filepath.Join(projectDir, KompoxDirName)
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
			got, err := searchForKompoxRoot(tt.startDir)
			if err != nil {
				t.Fatalf("searchForKompoxRoot() error: %v", err)
			}
			if got != tt.wantFound {
				t.Errorf("searchForKompoxRoot() = %q, want %q", got, tt.wantFound)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	kompoxDir := filepath.Join(projectDir, KompoxDirName)

	if err := os.MkdirAll(kompoxDir, 0755); err != nil {
		t.Fatalf("creating directories: %v", err)
	}

	tests := []struct {
		name       string
		kompoxRoot string
		kompoxDir  string
		workDir    string
		wantRoot   string
		wantDir    string
		wantErr    bool
	}{
		{
			name:       "explicit dirs",
			kompoxRoot: projectDir,
			kompoxDir:  kompoxDir,
			workDir:    tmpDir,
			wantRoot:   projectDir,
			wantDir:    kompoxDir,
			wantErr:    false,
		},
		{
			name:       "discover from workdir",
			kompoxRoot: "",
			kompoxDir:  "",
			workDir:    projectDir,
			wantRoot:   projectDir,
			wantDir:    kompoxDir,
			wantErr:    false,
		},
		{
			name:       "explicit kompoxRoot, default kompoxDir",
			kompoxRoot: projectDir,
			kompoxDir:  "",
			workDir:    tmpDir,
			wantRoot:   projectDir,
			wantDir:    kompoxDir,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Resolve(tt.kompoxRoot, tt.kompoxDir, tt.workDir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if cfg.KompoxRoot != tt.wantRoot {
				t.Errorf("KompoxRoot = %q, want %q", cfg.KompoxRoot, tt.wantRoot)
			}
			if cfg.KompoxDir != tt.wantDir {
				t.Errorf("KompoxDir = %q, want %q", cfg.KompoxDir, tt.wantDir)
			}
		})
	}
}

func TestConfig_ExpandVars(t *testing.T) {
	cfg := &Env{
		KompoxRoot: "/home/user/project",
		KompoxDir:  "/home/user/project/.kompox",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "expand KOMPOX_ROOT",
			input: "$KOMPOX_ROOT/kom",
			want:  "/home/user/project/kom",
		},
		{
			name:  "expand KOMPOX_DIR",
			input: "$KOMPOX_DIR/kom",
			want:  "/home/user/project/.kompox/kom",
		},
		{
			name:  "expand both",
			input: "$KOMPOX_ROOT/app and $KOMPOX_DIR/kom",
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
		KompoxRoot: "/home/user/project",
		KompoxDir:  "/home/user/project/.kompox",
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "within KOMPOX_ROOT",
			path: "/home/user/project/kom/app.yml",
			want: true,
		},
		{
			name: "within KOMPOX_DIR",
			path: "/home/user/project/.kompox/kom/db.yml",
			want: true,
		},
		{
			name: "exactly KOMPOX_ROOT",
			path: "/home/user/project",
			want: true,
		},
		{
			name: "exactly KOMPOX_DIR",
			path: "/home/user/project/.kompox",
			want: true,
		},
		{
			name: "outside boundary",
			path: "/home/user/other/kom/app.yml",
			want: false,
		},
		{
			name: "parent of KOMPOX_ROOT",
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
	kompoxDir := filepath.Join(tmpDir, KompoxDirName)
	if err := os.Mkdir(kompoxDir, 0755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}

	// Test case 1: config file exists
	configPath := filepath.Join(kompoxDir, ConfigFileName)
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
		KompoxRoot: tmpDir,
		KompoxDir:  kompoxDir,
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
	kompoxDir2 := filepath.Join(tmpDir2, KompoxDirName)
	if err := os.Mkdir(kompoxDir2, 0755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}

	cfg2 := &Env{
		KompoxRoot: tmpDir2,
		KompoxDir:  kompoxDir2,
	}

	if err := cfg2.loadConfigFile(); err != nil {
		t.Fatalf("loadConfigFile() error when file missing: %v", err)
	}

	// Should have zero values
	if cfg2.Version != 0 {
		t.Errorf("Version = %d, want 0 when config missing", cfg2.Version)
	}
}

func TestInitialConfigYAML(t *testing.T) {
	data, err := InitialConfigYAML()
	if err != nil {
		t.Fatalf("InitialConfigYAML() error: %v", err)
	}

	// Check that it's valid YAML
	if len(data) == 0 {
		t.Fatal("InitialConfigYAML() returned empty data")
	}

	// Expected content (with proper ordering and 2-space indent)
	expected := `version: 1
store:
  type: local
komPath:
  - kom
`
	got := string(data)
	if got != expected {
		t.Errorf("InitialConfigYAML() mismatch:\ngot:\n%s\nwant:\n%s", got, expected)
	}

	// Verify it can be unmarshaled back to configFile
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshaling generated YAML: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
	if cfg.Store.Type != "local" {
		t.Errorf("Store.Type = %q, want %q", cfg.Store.Type, "local")
	}
	if len(cfg.KOMPath) != 1 || cfg.KOMPath[0] != "kom" {
		t.Errorf("KOMPath = %v, want [\"kom\"]", cfg.KOMPath)
	}
}
