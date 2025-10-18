package v1alpha1

import (
	"regexp"
	"strings"
)

// HasLocalFSReference checks if an App uses local filesystem references.
// This includes:
// - file: prefix in spec.compose
// - Relative or absolute local paths in volume mounts (e.g., ./data:/data or /host/path:/container)
func HasLocalFSReference(app *App) bool {
	compose := app.Spec.Compose

	// Check for file: prefix
	if strings.Contains(compose, "file:") {
		return true
	}

	// Check for volume mount patterns:
	// - ./path:/container or ../path:/container (relative paths)
	// - /absolute/path:/container (absolute paths)
	// Pattern: line starting with whitespace, followed by "- ", then path with : separator
	// Examples:
	//   - ./data:/data
	//   - /var/lib/mysql:/var/lib/mysql
	//   - ../config:/config
	volumePattern := regexp.MustCompile(`(?m)^\s*-\s+(\.{1,2}/[^:]+|/[^:]+):[^:]+`)
	if volumePattern.MatchString(compose) {
		return true
	}

	return false
}
