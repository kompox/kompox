package kube

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/kompox/kompox/internal/logging"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	// Maximum size for ConfigMap data (1 MiB per spec)
	maxConfigMapSize = 1 << 20 // 1 MiB
	// Maximum length for config/secret names (DNS-1123 label limit)
	maxConfigSecretNameLength = 63
)

// NewComposeProject loads a compose project with RefBase-aware validation and resolution.
// RefBase controls external reference policy:
//   - "" (empty): external references are prohibited
//   - "file:///abs/dir/": local file references allowed, relative paths resolved from this directory
//   - "http(s)://.../": (reserved for future use, currently no effect)
//
// composeContent may be:
//   - inline YAML text (most common)
//   - "file:<path>" to load from file (requires RefBase with file:// scheme)
//
// Returns the loaded project and the working directory for subsequent file resolutions.
func NewComposeProject(ctx context.Context, composeContent, refBase string) (*types.Project, string, error) {
	logger := logging.FromContext(ctx)

	// Validate and parse refBase: must be empty string, or a valid URL with scheme
	var parsedBase *url.URL
	if refBase != "" {
		var err error
		parsedBase, err = url.Parse(refBase)
		if err != nil {
			return nil, "", fmt.Errorf("invalid RefBase %q: %w", refBase, err)
		}
		if parsedBase.Scheme == "" {
			return nil, "", fmt.Errorf("invalid RefBase %q: must have a scheme (file://, http://, https://)", refBase)
		}
		// Accept file://, http://, https:// schemes
		if parsedBase.Scheme != "file" && parsedBase.Scheme != "http" && parsedBase.Scheme != "https" {
			return nil, "", fmt.Errorf("invalid RefBase %q: unsupported scheme %q", refBase, parsedBase.Scheme)
		}
	}

	// Resolve compose content based on prefix and RefBase
	var content []byte
	var workingDir string // Actual directory for returning (used by converter for file resolution)
	var err error

	if strings.HasPrefix(composeContent, "file:") {
		// File reference: requires file:// RefBase and relative path only
		if parsedBase == nil || parsedBase.Scheme != "file" {
			return nil, "", fmt.Errorf("file: reference not allowed (RefBase: %q)", refBase)
		}
		relPath := strings.TrimPrefix(composeContent, "file:")

		// Reject absolute paths
		if filepath.IsAbs(relPath) {
			return nil, "", fmt.Errorf("file: reference must be relative path (got absolute path: %q)", relPath)
		}

		// Reject parent directory references
		if strings.Contains(relPath, "..") {
			return nil, "", fmt.Errorf("file: reference must not contain parent directory (..) references: %q", relPath)
		}

		baseDir := parsedBase.Path
		filePath := filepath.Join(baseDir, relPath)
		content, err = os.ReadFile(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("reading compose file %q: %w", filePath, err)
		}
		workingDir = filepath.Dir(filePath)

	} else {
		// Inline compose content
		content = []byte(composeContent)
		// Working directory depends on RefBase
		if parsedBase != nil && parsedBase.Scheme == "file" {
			workingDir = parsedBase.Path
		}
		// Otherwise workingDir remains empty (no local file access)
	}

	// Always use "." for compose-go WorkingDir to prevent it from resolving relative paths
	// We handle path resolution explicitly in Kompox using the workingDir return value
	cdm := types.ConfigDetails{
		WorkingDir:  ".",
		ConfigFiles: []types.ConfigFile{{Filename: "app.compose", Content: content}},
		Environment: map[string]string{},
	}
	model, err := loader.LoadModelWithContext(ctx, cdm, func(o *loader.Options) {
		o.SetProjectName("kompox-compose", false)
		o.SkipInclude = true
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to load compose model: %w", err)
	}
	if _, ok := model["version"]; ok {
		logger.Warn(ctx, "compose: `version` is obsolete")
	}
	var proj *types.Project
	if err := loader.Transform(model, &proj); err != nil {
		return nil, "", fmt.Errorf("failed to transform compose model to project: %w", err)
	}
	return proj, workingDir, nil
}

// validateConfigSecretName validates a config/secret name as a DNS-1123 label.
func validateConfigSecretName(name string) error {
	if name == "" {
		return fmt.Errorf("config/secret name must not be empty")
	}
	if len(name) > maxConfigSecretNameLength {
		return fmt.Errorf("config/secret name %q exceeds %d characters", name, maxConfigSecretNameLength)
	}
	if errs := utilvalidation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("invalid config/secret name %q: %s", name, strings.Join(errs, ", "))
	}
	return nil
}

// readFileContent reads a file and validates its size and encoding for ConfigMap/Secret.
// Requires RefBase with file:// scheme for local file access.
// For ConfigMap (isConfig=true): enforces UTF-8 without BOM and no NUL bytes, size ≤ 1 MiB.
// For Secret (isConfig=false): any binary content is allowed, size ≤ 1 MiB.
// Returns content bytes and a flag indicating if content is valid UTF-8 without NUL (suitable for data vs binaryData).
func readFileContent(baseDir, relPath string, isConfig bool, refBase string) ([]byte, bool, error) {
	// Validate RefBase allows file access
	parsedBase, err := url.Parse(refBase)
	if err != nil || parsedBase.Scheme != "file" {
		return nil, false, fmt.Errorf("local file reference not allowed (RefBase: %q): %s", refBase, relPath)
	}

	if strings.HasPrefix(relPath, "/") {
		return nil, false, fmt.Errorf("absolute path not allowed: %s", relPath)
	}
	if strings.Contains(relPath, "..") {
		return nil, false, fmt.Errorf("path traversal not allowed: %s", relPath)
	}
	fullPath := filepath.Join(baseDir, relPath)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, false, fmt.Errorf("stat failed: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("symlink not allowed: %s", relPath)
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("directory not allowed: %s", relPath)
	}
	if info.Size() > maxConfigMapSize {
		return nil, false, fmt.Errorf("file size %d exceeds limit %d (1 MiB): %s", info.Size(), maxConfigMapSize, relPath)
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, false, fmt.Errorf("read file: %w", err)
	}
	if len(content) > maxConfigMapSize {
		return nil, false, fmt.Errorf("file size %d exceeds limit %d (1 MiB): %s", len(content), maxConfigMapSize, relPath)
	}

	// Check UTF-8 validity and BOM/NUL for ConfigMap
	isValidUTF8Text := utf8.Valid(content) && !bytes.Contains(content, []byte{0})
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		isValidUTF8Text = false // BOM present
	}

	if isConfig && !isValidUTF8Text {
		return nil, false, fmt.Errorf("ConfigMap requires UTF-8 without BOM and no NUL bytes: %s", relPath)
	}

	return content, isValidUTF8Text, nil
}

// resolveConfigOrSecretFile resolves a config/secret definition to file path and content.
// Requires RefBase with file:// scheme for file references.
// Supports: file, content (inline), name (passthrough), external (treated as name passthrough).
// For file: reads from baseDir and validates.
// For content: uses inline content directly.
// For name/external: returns empty content (caller must handle as external reference).
// Returns: file basename (key), content bytes, isValidUTF8Text flag, error.
func resolveConfigOrSecretFile(baseDir, defName string, def types.FileObjectConfig, isConfig bool, refBase string) (string, []byte, bool, error) {
	// External or name-only: passthrough (no content)
	if def.External || (def.Name != "" && def.File == "" && def.Content == "") {
		return defName, nil, false, nil
	}

	// Content inline
	if def.Content != "" {
		content := []byte(def.Content)
		if len(content) > maxConfigMapSize {
			return "", nil, false, fmt.Errorf("inline content size %d exceeds limit %d (1 MiB): %s", len(content), maxConfigMapSize, defName)
		}
		isValidUTF8Text := utf8.Valid(content) && !bytes.Contains(content, []byte{0})
		if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
			isValidUTF8Text = false // BOM present
		}
		if isConfig && !isValidUTF8Text {
			return "", nil, false, fmt.Errorf("ConfigMap inline content requires UTF-8 without BOM and no NUL bytes: %s", defName)
		}
		return defName, content, isValidUTF8Text, nil
	}

	// File
	if def.File == "" {
		return "", nil, false, fmt.Errorf("config/secret %q must specify file, content, or name/external", defName)
	}
	content, isValidUTF8Text, err := readFileContent(baseDir, def.File, isConfig, refBase)
	if err != nil {
		return "", nil, false, fmt.Errorf("config/secret %q: %w", defName, err)
	}
	key := filepath.Base(def.File)
	return key, content, isValidUTF8Text, nil
}
