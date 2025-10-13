package main

import (
	"fmt"
	"os"
	"strings"

	crdv1 "github.com/kompox/kompox/config/crd/ops/v1alpha1"
	"github.com/spf13/cobra"
)

// crdModeContext holds state for CRD mode.
type crdModeContext struct {
	enabled          bool
	sink             *crdv1.Sink
	defaultAppID     string // FQN of the default app
	defaultClusterID string // FQN of the default cluster
}

// Global CRD mode context (set during initialization).
var crdMode crdModeContext

// getCRDPaths extracts CRD paths from flags and environment variables.
// Flags take precedence over environment variables.
func getCRDPaths(cmd *cobra.Command) []string {
	var paths []string

	// Get paths from flag
	flagPaths, err := cmd.Flags().GetStringArray("crd-path")
	if err == nil && len(flagPaths) > 0 {
		paths = append(paths, flagPaths...)
	}

	// If no flag paths, check environment variable
	if len(paths) == 0 {
		if envPaths := os.Getenv("KOMPOX_CRD_PATH"); envPaths != "" {
			for _, p := range strings.Split(envPaths, ",") {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					paths = append(paths, trimmed)
				}
			}
		}
	}

	return paths
}

// getCRDAppPath extracts the CRD app path from flags and environment variables.
// Flags take precedence over environment variables.
func getCRDAppPath(cmd *cobra.Command) string {
	// Check if flag was explicitly set
	if cmd.Flags().Changed("crd-app") {
		flagValue, _ := cmd.Flags().GetString("crd-app")
		return flagValue
	}

	// Check environment variable
	if envValue := os.Getenv("KOMPOX_CRD_APP"); envValue != "" {
		return envValue
	}

	// Default
	return "./kompoxapp.yml"
}

// initializeCRDMode attempts to load and validate CRD documents.
// If successful, it sets up the CRD mode context and returns true.
// If CRD inputs are not available or validation fails, returns false with an error if critical.
func initializeCRDMode(cmd *cobra.Command) error {
	crdPaths := getCRDPaths(cmd)
	crdAppPath := getCRDAppPath(cmd)

	// Collect all paths to load
	var allPaths []string
	allPaths = append(allPaths, crdPaths...)

	// Add crd-app path if it exists
	if crdAppPath != "" {
		if _, err := os.Stat(crdAppPath); err == nil {
			allPaths = append(allPaths, crdAppPath)
		}
		// If crd-app doesn't exist, silently ignore (per spec)
	}

	// If no paths, CRD mode is not applicable
	if len(allPaths) == 0 {
		return nil
	}

	// Validate that all crd-path entries exist (per spec)
	for _, p := range crdPaths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("--crd-path %q does not exist: %w", p, err)
		}
	}

	// Load documents from all paths
	loader := crdv1.NewLoader()
	var allDocuments []crdv1.Document
	var loadErrors []error

	for _, path := range allPaths {
		result, err := loader.Load(path)
		if err != nil {
			return fmt.Errorf("failed to load CRD from %q: %w", path, err)
		}
		allDocuments = append(allDocuments, result.Documents...)
		loadErrors = append(loadErrors, result.Errors...)
	}

	// If there were loading errors, fail
	if len(loadErrors) > 0 {
		return fmt.Errorf("CRD loading errors: %v", loadErrors)
	}

	// If no documents were loaded, CRD mode is not applicable
	if len(allDocuments) == 0 {
		return nil
	}

	// Validate documents and create sink
	sink, err := crdv1.NewSink(allDocuments)
	if err != nil {
		return fmt.Errorf("CRD validation failed: %w", err)
	}

	// CRD mode is now active
	crdMode.enabled = true
	crdMode.sink = sink

	// Infer default app name from crd-app if exactly one App exists
	if crdAppPath != "" {
		if _, err := os.Stat(crdAppPath); err == nil {
			// Load documents specifically from crd-app path
			appResult, err := loader.Load(crdAppPath)
			if err == nil && len(appResult.Documents) > 0 {
				// Count App documents
				appCount := 0
				var appName string
				var appParentPath string
				for _, doc := range appResult.Documents {
					if doc.Kind == "App" {
						appCount++
						if app, ok := doc.Object.(*crdv1.App); ok {
							appName = app.ObjectMeta.Name
							// Extract parent path (ws/prv/cls) from annotations
							if app.ObjectMeta.Annotations != nil {
								appParentPath = app.ObjectMeta.Annotations["ops.kompox.dev/path"]
							}
						}
					}
				}
				// If exactly one App, extract its FQN and cluster FQN from parent path
				if appCount == 1 && appName != "" {
					// Build App FQN (ws/prv/cls/app)
					if appParentPath != "" && appName != "" {
						if appFQN, err := crdv1.BuildFQN("App", appParentPath, appName); err == nil {
							crdMode.defaultAppID = appFQN.String()
						}
					}
					// Parent path IS the cluster FQN (ws/prv/cls)
					if appParentPath != "" {
						crdMode.defaultClusterID = appParentPath
					}
				}
			}
		}
	}

	return nil
}
