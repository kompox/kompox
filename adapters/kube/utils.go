package kube

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

// BuildCleanManifest converts runtime.Objects to unstructured maps, prunes empty maps / null values,
// drops some noisy fields, and returns a multi-document YAML string (each doc preceded by ---).
func BuildCleanManifest(objs []runtime.Object) (string, error) {
	var buf bytes.Buffer
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return "", fmt.Errorf("to unstructured: %w", err)
		}
		pruneMap(m)
		if meta, ok := m["metadata"].(map[string]any); ok { // drop empty creationTimestamp
			delete(meta, "creationTimestamp")
			if len(meta) == 0 {
				delete(m, "metadata")
			}
		}
		if st, ok := m["status"].(map[string]any); ok && len(st) == 0 { // drop empty status
			delete(m, "status")
		}
		var ybuf bytes.Buffer
		enc := yaml.NewEncoder(&ybuf)
		enc.SetIndent(2)
		if err := enc.Encode(m); err != nil {
			return "", err
		}
		_ = enc.Close()
		b := ybuf.Bytes()
		buf.WriteString("---\n")
		buf.Write(b)
		if len(b) == 0 || b[len(b)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	return buf.String(), nil
}

// pruneMap recursively prunes nil values and empty maps from a structure (in-place), preserving empty slices.
func pruneMap(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			cleaned := pruneMap(val)
			switch cv := cleaned.(type) {
			case nil:
				delete(x, k)
			case map[string]any:
				if len(cv) == 0 {
					delete(x, k)
				} else {
					x[k] = cv
				}
			default:
				x[k] = cv
			}
		}
		return x
	case []any:
		for i, it := range x {
			x[i] = pruneMap(it)
		}
		return x
	default:
		return x
	}
}

// tempfile writes arbitrary bytes to a temporary file and returns its path
// and a cleanup function to remove it.
func tempfile(bytes []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "kompox-kube-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp file: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(bytes); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("close temp file: %w", err)
	}
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}
