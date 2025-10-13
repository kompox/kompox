package v1alpha1

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Document represents a parsed CRD document with its FQN and original object.
type Document struct {
	Kind   string
	FQN    FQN
	Object any
	// Path is the file path from which this document was loaded.
	Path string
	// Index is the 1-based position of this document within its source file.
	// For multi-document YAML files, this indicates which document (1st, 2nd, etc.).
	Index int
}

// LoaderResult contains the results of loading CRD documents.
type LoaderResult struct {
	Documents []Document
	Errors    []error
}

// Loader loads CRD documents from files and directories.
type Loader struct {
	// MaxFileSize is the maximum file size in bytes to read (default: 10MB).
	MaxFileSize int64
}

// NewLoader creates a new Loader with default settings.
func NewLoader() *Loader {
	return &Loader{
		MaxFileSize: 10 * 1024 * 1024, // 10MB
	}
}

// Load loads CRD documents from the given path (file or directory).
// If the path is a directory, it recursively scans for .yml and .yaml files.
func (l *Loader) Load(path string) (*LoaderResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot stat path %q: %w", path, err)
	}

	if info.IsDir() {
		return l.loadDirectory(path)
	}
	return l.loadFile(path)
}

// loadDirectory recursively loads all .yml and .yaml files from a directory.
func (l *Loader) loadDirectory(dir string) (*LoaderResult, error) {
	result := &LoaderResult{
		Documents: make([]Document, 0),
		Errors:    make([]error, 0),
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("walk error at %q: %w", path, err))
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}

		fileResult, err := l.loadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("loading %q: %w", path, err))
			return nil // Continue walking
		}

		result.Documents = append(result.Documents, fileResult.Documents...)
		result.Errors = append(result.Errors, fileResult.Errors...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory %q: %w", dir, err)
	}

	return result, nil
}

// loadFile loads CRD documents from a single YAML file (supports multi-document).
func (l *Loader) loadFile(path string) (*LoaderResult, error) {
	result := &LoaderResult{
		Documents: make([]Document, 0),
		Errors:    make([]error, 0),
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", path, err)
	}
	defer file.Close()

	// Check file size
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file %q: %w", path, err)
	}
	if info.Size() > l.MaxFileSize {
		return nil, fmt.Errorf("file %q exceeds max size %d bytes", path, l.MaxFileSize)
	}

	// Create YAML decoder for multi-document support
	decoder := k8syaml.NewYAMLOrJSONDecoder(file, 4096)

	docIndex := 0
	for {
		docIndex++
		doc, err := l.decodeDocument(decoder, path, docIndex)
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if doc != nil {
			result.Documents = append(result.Documents, *doc)
		}
	}

	return result, nil
}

// decodeDocument decodes a single document from the YAML stream.
func (l *Loader) decodeDocument(decoder *k8syaml.YAMLOrJSONDecoder, path string, docIndex int) (*Document, error) {
	// First decode into a generic map to inspect apiVersion and kind
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("decoding document %d in %q: %w", docIndex, path, err)
	}

	// Skip empty documents
	if len(raw) == 0 {
		return nil, nil
	}

	apiVersion, ok := raw["apiVersion"].(string)
	if !ok {
		return nil, fmt.Errorf("document %d in %q: missing or invalid apiVersion", docIndex, path)
	}

	kind, ok := raw["kind"].(string)
	if !ok {
		return nil, fmt.Errorf("document %d in %q: missing or invalid kind", docIndex, path)
	}

	// Check if this is a Kompox CRD
	expectedAPIVersion := Group + "/" + Version
	if apiVersion != expectedAPIVersion {
		// Skip non-Kompox documents silently
		return nil, nil
	}

	// Parse the document based on kind
	doc, err := l.parseKindDocument(raw, kind, path, docIndex)
	if err != nil {
		return nil, fmt.Errorf("document %d in %q: %w", docIndex, path, err)
	}

	return doc, nil
}

// parseKindDocument parses a document of a specific kind and builds its FQN.
func (l *Loader) parseKindDocument(raw map[string]any, kind string, path string, docIndex int) (*Document, error) {
	// Extract metadata
	metadataRaw, ok := raw["metadata"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid metadata")
	}

	name, ok := metadataRaw["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing or invalid metadata.name")
	}

	// Extract annotations
	var annotations map[string]string
	if annotationsRaw, ok := metadataRaw["annotations"].(map[string]any); ok {
		annotations = make(map[string]string)
		for k, v := range annotationsRaw {
			if strVal, ok := v.(string); ok {
				annotations[k] = strVal
			}
		}
	}

	// Extract parent path from annotations
	parentPath, err := ExtractParentPath(kind, annotations)
	if err != nil {
		return nil, err
	}

	// Build FQN
	fqn, err := BuildFQN(kind, parentPath, name)
	if err != nil {
		return nil, err
	}

	// Parse the document into the appropriate type
	var obj any
	switch kind {
	case "Workspace":
		var ws Workspace
		if err := mapToStruct(raw, &ws); err != nil {
			return nil, fmt.Errorf("parsing Workspace: %w", err)
		}
		obj = &ws
	case "Provider":
		var prv Provider
		if err := mapToStruct(raw, &prv); err != nil {
			return nil, fmt.Errorf("parsing Provider: %w", err)
		}
		obj = &prv
	case "Cluster":
		var cls Cluster
		if err := mapToStruct(raw, &cls); err != nil {
			return nil, fmt.Errorf("parsing Cluster: %w", err)
		}
		obj = &cls
	case "App":
		var app App
		if err := mapToStruct(raw, &app); err != nil {
			return nil, fmt.Errorf("parsing App: %w", err)
		}
		obj = &app
	case "Box":
		var box Box
		if err := mapToStruct(raw, &box); err != nil {
			return nil, fmt.Errorf("parsing Box: %w", err)
		}
		obj = &box
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}

	return &Document{
		Kind:   kind,
		FQN:    fqn,
		Object: obj,
		Path:   path,
		Index:  docIndex,
	}, nil
}

// mapToStruct converts a map[string]any to a struct using JSON marshaling/unmarshaling.
// This approach preserves JSON tags used in Kubernetes types.
func mapToStruct(m map[string]any, target any) error {
	// Convert to JSON bytes
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling to JSON: %w", err)
	}

	// Unmarshal into target
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("unmarshaling to struct: %w", err)
	}

	return nil
}
