package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	komv1 "github.com/kompox/kompox/config/crd/ops/v1alpha1"
	"github.com/spf13/cobra"
)

// KOM loader limits
const (
	maxKOMFiles     = 5000
	maxKOMFileSize  = 2 * 1024 * 1024  // 2 MiB
	maxKOMTotalSize = 32 * 1024 * 1024 // 32 MiB
	maxKOMDepth     = 10
)

// Paths to ignore during recursive directory scanning
var ignoredPathComponents = []string{
	".git", ".github", "node_modules", "vendor", ".direnv", ".venv", "dist", "build",
}

// findBaseRoot searches for a base root directory by looking for .git or .kompoxroot
// in ancestors of the given directory. Returns the directory containing the marker,
// or the parent directory of dir if no marker is found.
func findBaseRoot(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving directory path: %w", err)
	}

	current := absDir
	for {
		// Check for .git or .kompoxroot
		gitPath := filepath.Join(current, ".git")
		kompoxrootPath := filepath.Join(current, ".kompoxroot")

		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}
		if _, err := os.Stat(kompoxrootPath); err == nil {
			return current, nil
		}

		// Move to parent
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding marker
			// Return the parent of the original directory
			return filepath.Dir(absDir), nil
		}
		current = parent
	}
}

// isPathIgnored checks if any component of the path should be ignored.
func isPathIgnored(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		for _, ignored := range ignoredPathComponents {
			if part == ignored {
				return true
			}
		}
	}
	return false
}

// validateAndResolveKOMPath validates a KOM path according to spec requirements:
// - Local paths only (no URLs)
// - Resolves to absolute path with symlinks evaluated
// - Must be within baseRoot
// - Must have .yml or .yaml extension (if file)
func validateAndResolveKOMPath(inputPath, baseDir, baseRoot string) (string, error) {
	// Reject URLs
	if strings.Contains(inputPath, "://") {
		return "", fmt.Errorf("URL not supported: %q", inputPath)
	}

	// Resolve relative paths relative to baseDir
	resolvedPath := inputPath
	if !filepath.IsAbs(inputPath) {
		resolvedPath = filepath.Join(baseDir, inputPath)
	}

	// Clean the path
	resolvedPath = filepath.Clean(resolvedPath)

	// Evaluate symlinks to get real path
	realPath, err := filepath.EvalSymlinks(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %q", inputPath)
		}
		return "", fmt.Errorf("resolving path %q: %w", inputPath, err)
	}

	// Ensure the path is within baseRoot
	relPath, err := filepath.Rel(baseRoot, realPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path %q (resolved to %q) is outside base root %q", inputPath, realPath, baseRoot)
	}

	// Check if it's a file or directory
	info, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("stat failed for %q: %w", realPath, err)
	}

	// If it's a file, validate extension
	if !info.IsDir() {
		ext := filepath.Ext(realPath)
		if ext != ".yml" && ext != ".yaml" {
			return "", fmt.Errorf("file %q must have .yml or .yaml extension", inputPath)
		}
	}

	return realPath, nil
}

// scanKOMDirectory recursively scans a directory for .yml/.yaml files,
// respecting ignore patterns and limits.
func scanKOMDirectory(dir string, baseRoot string, depth int, visited map[string]bool, stats *komScanStats) ([]string, error) {
	if depth > maxKOMDepth {
		return nil, fmt.Errorf("maximum directory depth (%d) exceeded", maxKOMDepth)
	}

	// Get canonical path to avoid duplicate processing
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("evaluating symlinks for %q: %w", dir, err)
	}

	if visited[canonicalDir] {
		return nil, nil // Already processed
	}
	visited[canonicalDir] = true

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dir, err)
	}

	var files []string

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		// Check ignore patterns
		if isPathIgnored(fullPath) {
			continue
		}

		if entry.IsDir() {
			// Recurse into subdirectory
			subFiles, err := scanKOMDirectory(fullPath, baseRoot, depth+1, visited, stats)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else {
			// Check extension
			ext := filepath.Ext(entry.Name())
			if ext != ".yml" && ext != ".yaml" {
				continue
			}

			// Get file info for size check
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("getting info for %q: %w", fullPath, err)
			}

			fileSize := info.Size()

			// Check limits
			if stats.fileCount >= maxKOMFiles {
				return nil, fmt.Errorf("maximum file count (%d) exceeded", maxKOMFiles)
			}
			if fileSize > maxKOMFileSize {
				return nil, fmt.Errorf("file %q exceeds maximum size (%d bytes)", fullPath, maxKOMFileSize)
			}
			if stats.totalSize+fileSize > maxKOMTotalSize {
				return nil, fmt.Errorf("total size limit (%d bytes) exceeded", maxKOMTotalSize)
			}

			stats.fileCount++
			stats.totalSize += fileSize

			files = append(files, fullPath)
		}
	}

	return files, nil
}

// komScanStats tracks statistics during directory scanning
type komScanStats struct {
	fileCount int
	totalSize int64
}

// komModeContext holds state for KOM mode.
type komModeContext struct {
	enabled          bool
	sink             *komv1.Sink
	defaultAppID     string // FQN of the default app
	defaultClusterID string // FQN of the default cluster
}

// Global KOM mode context (set during initialization).
var komMode komModeContext

// getKOMPaths extracts KOM paths from flags and environment variables.
// Flags take precedence over environment variables.
func getKOMPaths(cmd *cobra.Command) []string {
	var paths []string

	// Get paths from flag
	flagPaths, err := cmd.Flags().GetStringArray("kom-path")
	if err == nil && len(flagPaths) > 0 {
		paths = append(paths, flagPaths...)
	}

	// If no flag paths, check environment variable
	if len(paths) == 0 {
		if envPaths := os.Getenv("KOMPOX_KOM_PATH"); envPaths != "" {
			for _, p := range strings.Split(envPaths, ",") {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					paths = append(paths, trimmed)
				}
			}
		}
	}

	return paths
}

// getKOMAppPath extracts the KOM app path from flags and environment variables.
// Flags take precedence over environment variables.
func getKOMAppPath(cmd *cobra.Command) string {
	// Check if flag was explicitly set
	if cmd.Flags().Changed("kom-app") {
		flagValue, _ := cmd.Flags().GetString("kom-app")
		return flagValue
	}

	// Check environment variable
	if envValue := os.Getenv("KOMPOX_KOM_APP"); envValue != "" {
		return envValue
	}

	// Default
	return "./kompoxapp.yml"
}

// initializeKOMMode attempts to load and validate KOM documents.
// If successful, it sets up the KOM mode context.
func initializeKOMMode(cmd *cobra.Command) error {
	komPaths := getKOMPaths(cmd)
	komAppPath := getKOMAppPath(cmd)

	loader := komv1.NewLoader()

	// Load kompoxapp.yml to extract Defaults (if present)
	var defaults *komv1.Defaults
	var komAppDocs []komv1.Document
	var komAppDir string
	var baseRoot string

	// Determine komAppDir and baseRoot
	if komAppPath != "" {
		komAppDir = filepath.Dir(komAppPath)
		if !filepath.IsAbs(komAppDir) {
			var err error
			komAppDir, err = filepath.Abs(komAppDir)
			if err != nil {
				return fmt.Errorf("resolving kompoxapp.yml directory: %w", err)
			}
		}

		// Find base root for validation
		var err error
		baseRoot, err = findBaseRoot(komAppDir)
		if err != nil {
			return fmt.Errorf("finding base root: %w", err)
		}

		// Load kompoxapp.yml if it exists
		if _, err := os.Stat(komAppPath); err == nil {
			result, err := loader.Load(komAppPath)
			if err != nil {
				return fmt.Errorf("failed to load kompoxapp.yml from %q: %w", komAppPath, err)
			}
			if len(result.Errors) > 0 {
				return fmt.Errorf("errors loading kompoxapp.yml: %v", result.Errors)
			}

			// Extract Defaults (at most one allowed)
			defaultsCount := 0
			for _, doc := range result.Documents {
				if doc.Kind == "Defaults" {
					defaultsCount++
					if defaultsCount > 1 {
						return fmt.Errorf("multiple Defaults documents found in %q; only one is allowed", komAppPath)
					}
					if d, ok := doc.Object.(*komv1.Defaults); ok {
						defaults = d
					}
				} else {
					// Collect non-Defaults documents
					komAppDocs = append(komAppDocs, doc)
				}
			}
		}
		// If kom-app doesn't exist, silently ignore (per spec)
	} else {
		// No kompoxapp.yml specified, use current directory
		var err error
		komAppDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		baseRoot, err = findBaseRoot(komAppDir)
		if err != nil {
			return fmt.Errorf("finding base root: %w", err)
		}
	}

	// Collect and validate all KOM paths
	var validatedKOMPaths []string

	// Priority 1: --kom-path / KOMPOX_KOM_PATH
	for _, p := range komPaths {
		validated, err := validateAndResolveKOMPath(p, komAppDir, baseRoot)
		if err != nil {
			return fmt.Errorf("--kom-path validation failed: %w", err)
		}
		validatedKOMPaths = append(validatedKOMPaths, validated)
	}

	// Priority 2: Defaults.spec.komPath (if no --kom-path)
	if len(komPaths) == 0 && defaults != nil && len(defaults.Spec.KOMPath) > 0 {
		for _, p := range defaults.Spec.KOMPath {
			validated, err := validateAndResolveKOMPath(p, komAppDir, baseRoot)
			if err != nil {
				return fmt.Errorf("Defaults.spec.komPath validation failed: %w", err)
			}
			validatedKOMPaths = append(validatedKOMPaths, validated)
		}
	}

	// Expand directories to file lists and deduplicate
	var allKOMFiles []string
	visitedFiles := make(map[string]bool)
	visitedDirs := make(map[string]bool)
	stats := &komScanStats{}

	for _, path := range validatedKOMPaths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat failed for %q: %w", path, err)
		}

		if info.IsDir() {
			// Scan directory recursively
			files, err := scanKOMDirectory(path, baseRoot, 0, visitedDirs, stats)
			if err != nil {
				return fmt.Errorf("scanning directory %q: %w", path, err)
			}
			for _, f := range files {
				if !visitedFiles[f] {
					visitedFiles[f] = true
					allKOMFiles = append(allKOMFiles, f)
				}
			}
		} else {
			// Single file
			if !visitedFiles[path] {
				// Check size limit
				if info.Size() > maxKOMFileSize {
					return fmt.Errorf("file %q exceeds maximum size (%d bytes)", path, maxKOMFileSize)
				}
				if stats.totalSize+info.Size() > maxKOMTotalSize {
					return fmt.Errorf("total size limit (%d bytes) exceeded", maxKOMTotalSize)
				}
				if stats.fileCount >= maxKOMFiles {
					return fmt.Errorf("maximum file count (%d) exceeded", maxKOMFiles)
				}

				stats.fileCount++
				stats.totalSize += info.Size()

				visitedFiles[path] = true
				allKOMFiles = append(allKOMFiles, path)
			}
		}
	}

	// Load documents from all collected KOM files
	var allDocuments []komv1.Document
	var loadErrors []error

	for _, path := range allKOMFiles {
		result, err := loader.Load(path)
		if err != nil {
			return fmt.Errorf("failed to load KOM from %q: %w", path, err)
		}
		for _, doc := range result.Documents {
			// Skip Defaults documents from KOM paths
			if doc.Kind != "Defaults" {
				allDocuments = append(allDocuments, doc)
			}
		}
		loadErrors = append(loadErrors, result.Errors...)
	}

	// Add kompoxapp.yml documents (non-Defaults)
	allDocuments = append(allDocuments, komAppDocs...)

	// If there were loading errors, fail
	if len(loadErrors) > 0 {
		return fmt.Errorf("KOM loading errors: %v", loadErrors)
	}

	// If no documents were loaded, KOM mode is not applicable
	if len(allDocuments) == 0 {
		return nil
	}

	// Validate documents and create sink
	sink, err := komv1.NewSink(allDocuments)
	if err != nil {
		return fmt.Errorf("KOM validation failed: %w", err)
	}

	// Validate local filesystem reference constraints
	// Apps directly in kompoxapp.yml can use local FS references
	// Apps from other sources cannot
	komAppAbsPath := ""
	if komAppPath != "" {
		if abs, err := filepath.Abs(komAppPath); err == nil {
			komAppAbsPath = abs
		}
	}

	for _, doc := range allDocuments {
		if doc.Kind == "App" {
			app, ok := doc.Object.(*komv1.App)
			if !ok {
				continue
			}

			// Check if this App uses local FS references
			if komv1.HasLocalFSReference(app) {
				// Get the document path
				docPath := ""
				if app.ObjectMeta.Annotations != nil {
					docPath = app.ObjectMeta.Annotations[komv1.AnnotationDocPath]
				}

				// Resolve to absolute path
				var docAbsPath string
				if docPath != "" {
					if abs, err := filepath.Abs(docPath); err == nil {
						docAbsPath = abs
					}
				}

				// Check if this App is from kompoxapp.yml
				isFromKomApp := (komAppAbsPath != "" && docAbsPath == komAppAbsPath)

				if !isFromKomApp {
					return fmt.Errorf("App %q uses local filesystem references but is not defined in kompoxapp.yml (defined in %q)", doc.FQN, docPath)
				}
			}
		}
	}

	// KOM mode is now active
	komMode.enabled = true
	komMode.sink = sink

	// Determine default App ID
	// Priority: --app-id > Defaults.spec.appId > single App in kompoxapp.yml
	if defaults != nil && defaults.Spec.AppID != "" {
		// Validate the AppID format
		if appFQN, _, err := komv1.ParseResourceID(defaults.Spec.AppID); err == nil {
			komMode.defaultAppID = appFQN.String()
			// Extract parent cluster ID from App FQN
			komMode.defaultClusterID = appFQN.ParentFQN().String()
		} else {
			return fmt.Errorf("invalid Defaults.spec.appId %q: %w", defaults.Spec.AppID, err)
		}
	} else {
		// Fallback: if exactly one App in kompoxapp.yml, use it
		appCount := 0
		var appResID string
		for _, doc := range komAppDocs {
			if doc.Kind == "App" {
				appCount++
				if app, ok := doc.Object.(*komv1.App); ok {
					if app.ObjectMeta.Annotations != nil {
						appResID = app.ObjectMeta.Annotations[komv1.AnnotationID]
					}
				}
			}
		}
		if appCount == 1 && appResID != "" {
			if appFQN, _, err := komv1.ParseResourceID(appResID); err == nil {
				komMode.defaultAppID = appFQN.String()
				komMode.defaultClusterID = appFQN.ParentFQN().String()
			}
		}
	}

	return nil
}
