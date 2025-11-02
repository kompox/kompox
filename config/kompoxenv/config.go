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
	KompoxDirEnvKey    = "KOMPOX_DIR"
	KompoxCfgDirEnvKey = "KOMPOX_CFG_DIR"
)

// Directory and file names
const (
	KompoxCfgDirName = ".kompox"
	ConfigFileName   = "config.yml"
)

// Env holds the resolved KOMPOX_DIR, KOMPOX_CFG_DIR, and loaded .kompox/config.yml contents.
// It represents the Kompox project environment and provides utilities for path expansion
// and boundary checking.
type Env struct {
	KompoxDir    string   // Resolved KOMPOX_DIR
	KompoxCfgDir string   // Resolved KOMPOX_CFG_DIR
	Version      int      // .kompox/config.yml version
	Store        Store    // .kompox/config.yml store configuration
	KOMPath      []string // .kompox/config.yml komPath
}

// Store represents the store configuration from .kompox/config.yml
type Store struct {
	Type string `yaml:"type"` // local | rdb | custom
}

// configFile represents the structure of .kompox/config.yml for unmarshaling
type configFile struct {
	Version int      `yaml:"version"`
	Store   Store    `yaml:"store"`
	KOMPath []string `yaml:"komPath,omitempty"`
}

// Resolve discovers KOMPOX_DIR and KOMPOX_CFG_DIR, then loads .kompox/config.yml.
//
// Resolution order for KOMPOX_DIR:
//  1. kompoxDir parameter (from --kompox-dir flag or KOMPOX_DIR env)
//  2. Upward search from workDir for parent containing .kompox/
//
// Resolution order for KOMPOX_CFG_DIR:
//  1. kompoxCfgDir parameter (from --kompox-cfg-dir flag or KOMPOX_CFG_DIR env)
//  2. Default: $KOMPOX_DIR/.kompox
//
// Parameters can be empty strings to trigger discovery/defaults.
func Resolve(kompoxDir, kompoxCfgDir, workDir string) (*Env, error) {
	// Resolve KOMPOX_DIR
	if kompoxDir == "" {
		// Search upward for .kompox directory
		found, err := searchForKompoxDir(workDir)
		if err != nil {
			return nil, fmt.Errorf("searching for .kompox directory: %w", err)
		}
		if found == "" {
			return nil, fmt.Errorf("KOMPOX_DIR not specified and .kompox directory not found in ancestors of %q", workDir)
		}
		kompoxDir = found
	}

	// Make absolute and clean
	var err error
	kompoxDir, err = filepath.Abs(kompoxDir)
	if err != nil {
		return nil, fmt.Errorf("resolving KOMPOX_DIR to absolute path: %w", err)
	}
	kompoxDir = filepath.Clean(kompoxDir)

	// Verify it's a directory
	info, err := os.Stat(kompoxDir)
	if err != nil {
		return nil, fmt.Errorf("KOMPOX_DIR %q does not exist: %w", kompoxDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("KOMPOX_DIR %q is not a directory", kompoxDir)
	}

	// Resolve KOMPOX_CFG_DIR
	if kompoxCfgDir == "" {
		// Default: $KOMPOX_DIR/.kompox
		kompoxCfgDir = filepath.Join(kompoxDir, KompoxCfgDirName)
	}

	// Make absolute and clean
	kompoxCfgDir, err = filepath.Abs(kompoxCfgDir)
	if err != nil {
		return nil, fmt.Errorf("resolving KOMPOX_CFG_DIR to absolute path: %w", err)
	}
	kompoxCfgDir = filepath.Clean(kompoxCfgDir)

	// Verify it's a directory
	info, err = os.Stat(kompoxCfgDir)
	if err != nil {
		return nil, fmt.Errorf("KOMPOX_CFG_DIR %q does not exist: %w", kompoxCfgDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("KOMPOX_CFG_DIR %q is not a directory", kompoxCfgDir)
	}

	// Create Config
	cfg := &Env{
		KompoxDir:    kompoxDir,
		KompoxCfgDir: kompoxCfgDir,
	}

	// Load .kompox/config.yml if it exists
	if err := cfg.loadConfigFile(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// searchForKompoxDir searches upward from startDir for a parent containing .kompox directory.
// Returns the parent directory (not .kompox itself) or empty string if not found.
func searchForKompoxDir(startDir string) (string, error) {
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving start directory: %w", err)
	}

	current := absDir
	for {
		// Check for .kompox subdirectory
		kompoxPath := filepath.Join(current, KompoxCfgDirName)
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
	configPath := filepath.Join(e.KompoxCfgDir, ConfigFileName)

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

	return nil
}

// ExpandVars replaces $KOMPOX_DIR and $KOMPOX_CFG_DIR in the given string.
func (e *Env) ExpandVars(s string) string {
	s = strings.ReplaceAll(s, "$KOMPOX_DIR", e.KompoxDir)
	s = strings.ReplaceAll(s, "$KOMPOX_CFG_DIR", e.KompoxCfgDir)
	return s
}

// IsWithinBoundary checks if the given path is within KOMPOX_DIR or KOMPOX_CFG_DIR.
// This is used for boundary validation of Defaults.spec.komPath.
// The path should be an absolute path (already resolved).
func (e *Env) IsWithinBoundary(path string) bool {
	cleanPath := filepath.Clean(path)

	// Check if within KOMPOX_DIR
	relToDir, err := filepath.Rel(e.KompoxDir, cleanPath)
	if err == nil && !strings.HasPrefix(relToDir, "..") && relToDir != ".." {
		return true
	}

	// Check if within KOMPOX_CFG_DIR
	relToCfgDir, err := filepath.Rel(e.KompoxCfgDir, cleanPath)
	if err == nil && !strings.HasPrefix(relToCfgDir, "..") && relToCfgDir != ".." {
		return true
	}

	return false
}
