package main

import (
	"fmt"
	"os"
	"strings"

	komv1 "github.com/kompox/kompox/config/crd/ops/v1alpha1"
	"github.com/spf13/cobra"
)

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

	// Collect all paths to load
	var allPaths []string
	allPaths = append(allPaths, komPaths...)

	// Add kom-app path if it exists
	if komAppPath != "" {
		if _, err := os.Stat(komAppPath); err == nil {
			allPaths = append(allPaths, komAppPath)
		}
		// If kom-app doesn't exist, silently ignore (per spec)
	}

	// If no paths, KOM mode is not applicable
	if len(allPaths) == 0 {
		return nil
	}

	// Validate that all kom-path entries exist (per spec)
	for _, p := range komPaths {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("--kom-path %q does not exist: %w", p, err)
		}
	}

	// Load documents from all paths
	loader := komv1.NewLoader()
	var allDocuments []komv1.Document
	var loadErrors []error

	for _, path := range allPaths {
		result, err := loader.Load(path)
		if err != nil {
			return fmt.Errorf("failed to load KOM from %q: %w", path, err)
		}
		allDocuments = append(allDocuments, result.Documents...)
		loadErrors = append(loadErrors, result.Errors...)
	}

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

	// KOM mode is now active
	komMode.enabled = true
	komMode.sink = sink

	// Infer default app name from kom-app if exactly one App exists
	if komAppPath != "" {
		if _, err := os.Stat(komAppPath); err == nil {
			// Load documents specifically from kom-app path
			appResult, err := loader.Load(komAppPath)
			if err == nil && len(appResult.Documents) > 0 {
				// Count App documents
				appCount := 0
				var appName string
				var appResID string
				for _, doc := range appResult.Documents {
					if doc.Kind == "App" {
						appCount++
						if app, ok := doc.Object.(*komv1.App); ok {
							appName = app.ObjectMeta.Name
							// Extract Resource ID from annotations
							if app.ObjectMeta.Annotations != nil {
								appResID = app.ObjectMeta.Annotations["ops.kompox.dev/id"]
							}
						}
					}
				}
				// If exactly one App, extract its Resource ID and parent cluster ID
				if appCount == 1 && appName != "" && appResID != "" {
					// Parse the App Resource ID
					if appFQN, _, err := komv1.ParseResourceID(appResID); err == nil {
						komMode.defaultAppID = appFQN.String()
						// Parent Resource ID is the cluster ID
						komMode.defaultClusterID = appFQN.ParentFQN().String()
					}
				}
			}
		}
	}

	return nil
}
