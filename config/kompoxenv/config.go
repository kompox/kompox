package kompoxenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Environment variable names
const (
	KompoxRootEnvKey = "KOMPOX_ROOT"
	KompoxDirEnvKey  = "KOMPOX_DIR"
)

// Directory and file names
const (
	KompoxDirName  = ".kompox"
	ConfigFileName = "config.yml"
)

// Env holds the resolved KOMPOX_ROOT, KOMPOX_DIR, and loaded .kompox/config.yml contents.
// It represents the Kompox project environment and provides utilities for path expansion
// and boundary checking.
type Env struct {
	KompoxRoot string   // Resolved KOMPOX_ROOT (project directory)
	KompoxDir  string   // Resolved KOMPOX_DIR (Kompox directory, typically .kompox)
	Version    int      // .kompox/config.yml version
	Store      Store    // .kompox/config.yml store configuration
	KOMPath    []string // .kompox/config.yml komPath
	Logging    Logging  // .kompox/config.yml logging configuration
}

// Store represents the store configuration from .kompox/config.yml
type Store struct {
	Type string `yaml:"type"` // local | rdb | custom
}

// Logging represents the logging configuration from .kompox/config.yml
type Logging struct {
	Dir           string `yaml:"dir,omitempty"`           // Log directory (default: $KOMPOX_DIR/logs)
	Format        string `yaml:"format,omitempty"`        // Log format: json (default), human
	Level         string `yaml:"level,omitempty"`         // Log level: DEBUG, INFO (default), WARN, ERROR
	RetentionDays int    `yaml:"retentionDays,omitempty"` // Days to retain log files (default: 7)
}

// configFile represents the structure of .kompox/config.yml for unmarshaling
type configFile struct {
	Version int      `yaml:"version"`
	Store   Store    `yaml:"store"`
	KOMPath []string `yaml:"komPath,omitempty"`
	Logging Logging  `yaml:"logging,omitempty"`
}

// Resolve discovers KOMPOX_ROOT and KOMPOX_DIR, then loads .kompox/config.yml.
//
// Resolution order for KOMPOX_ROOT:
//  1. kompoxRoot parameter (from --kompox-root flag or KOMPOX_ROOT env)
//  2. Upward search from workDir for parent containing .kompox/
//
// Resolution order for KOMPOX_DIR:
//  1. kompoxDir parameter (from --kompox-dir flag or KOMPOX_DIR env)
//  2. Default: $KOMPOX_ROOT/.kompox
//
// Parameters can be empty strings to trigger discovery/defaults.
func Resolve(kompoxRoot, kompoxDir, workDir string) (*Env, error) {
	// Resolve KOMPOX_ROOT
	if kompoxRoot == "" {
		// Search upward for .kompox directory
		found, err := searchForKompoxRoot(workDir)
		if err != nil {
			return nil, fmt.Errorf("searching for .kompox directory: %w", err)
		}
		if found == "" {
			return nil, fmt.Errorf("KOMPOX_ROOT not specified and .kompox directory not found in ancestors of %q", workDir)
		}
		kompoxRoot = found
	}

	// Make absolute and clean
	var err error
	kompoxRoot, err = filepath.Abs(kompoxRoot)
	if err != nil {
		return nil, fmt.Errorf("resolving KOMPOX_ROOT to absolute path: %w", err)
	}
	kompoxRoot = filepath.Clean(kompoxRoot)

	// Verify it's a directory
	info, err := os.Stat(kompoxRoot)
	if err != nil {
		return nil, fmt.Errorf("KOMPOX_ROOT %q does not exist: %w", kompoxRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("KOMPOX_ROOT %q is not a directory", kompoxRoot)
	}

	// Resolve KOMPOX_DIR
	if kompoxDir == "" {
		// Default: $KOMPOX_ROOT/.kompox
		kompoxDir = filepath.Join(kompoxRoot, KompoxDirName)
	}

	// Make absolute and clean
	kompoxDir, err = filepath.Abs(kompoxDir)
	if err != nil {
		return nil, fmt.Errorf("resolving KOMPOX_DIR to absolute path: %w", err)
	}
	kompoxDir = filepath.Clean(kompoxDir)

	// Verify it's a directory
	info, err = os.Stat(kompoxDir)
	if err != nil {
		return nil, fmt.Errorf("KOMPOX_DIR %q does not exist: %w", kompoxDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("KOMPOX_DIR %q is not a directory", kompoxDir)
	}

	// Create Env
	cfg := &Env{
		KompoxRoot: kompoxRoot,
		KompoxDir:  kompoxDir,
	}

	// Load .kompox/config.yml if it exists
	if err := cfg.loadConfigFile(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// searchForKompoxRoot searches upward from startDir for a parent containing .kompox directory.
// Returns the parent directory (not .kompox itself) or empty string if not found.
func searchForKompoxRoot(startDir string) (string, error) {
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory: %w", err)
	}

	current := absDir
	for {
		// Check for .kompox subdirectory
		kompoxPath := filepath.Join(current, KompoxDirName)
		info, err := os.Stat(kompoxPath)
		if err == nil && info.IsDir() {
			return current, nil
		}

		// Move to parent
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding .kompox
			return "", nil
		}
		current = parent
	}
}

// loadConfigFile loads .kompox/config.yml into the Env.
// Does nothing if the file doesn't exist (not an error).
func (e *Env) loadConfigFile() error {
	configPath := filepath.Join(e.KompoxDir, ConfigFileName)

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Not an error if config doesn't exist
	}

	// Read the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file %q: %w", configPath, err)
	}

	// Parse YAML
	var cf configFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("parsing config file %q: %w", configPath, err)
	}

	// Copy fields to Env
	e.Version = cf.Version
	e.Store = cf.Store
	e.KOMPath = cf.KOMPath
	e.Logging = cf.Logging

	return nil
}

// ExpandVars replaces $KOMPOX_ROOT and $KOMPOX_DIR in the given string.
func (e *Env) ExpandVars(s string) string {
	s = strings.ReplaceAll(s, "$KOMPOX_ROOT", e.KompoxRoot)
	s = strings.ReplaceAll(s, "$KOMPOX_DIR", e.KompoxDir)
	return s
}

// IsWithinBoundary checks if the given path is within KOMPOX_ROOT or KOMPOX_DIR.
// This is used for boundary validation of Defaults.spec.komPath.
// The path should be an absolute path (already resolved).
func (e *Env) IsWithinBoundary(path string) bool {
	cleanPath := filepath.Clean(path)

	// Check if within KOMPOX_ROOT
	relToRoot, err := filepath.Rel(e.KompoxRoot, cleanPath)
	if err == nil && !strings.HasPrefix(relToRoot, "..") && relToRoot != ".." {
		return true
	}

	// Check if within KOMPOX_DIR
	relToDir, err := filepath.Rel(e.KompoxDir, cleanPath)
	if err == nil && !strings.HasPrefix(relToDir, "..") && relToDir != ".." {
		return true
	}

	return false
}

// InitialConfigYAML generates the initial .kompox/config.yml content as YAML bytes.
// The generated YAML has proper field ordering and 2-space indentation.
func InitialConfigYAML() ([]byte, error) {
	defaultConfig := configFile{
		Version: 1,
		Store: Store{
			Type: "local",
		},
		// KOMPath is omitted - let Level 5 default ($KOMPOX_DIR/kom) be used
	}

	var buf strings.Builder
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&defaultConfig); err != nil {
		return nil, fmt.Errorf("encoding default config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("closing yaml encoder: %w", err)
	}

	return []byte(buf.String()), nil
}
