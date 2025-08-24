package kube

import (
	"bytes"
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

// applyXKompoxResources parses x-kompox extensions and sets container resources.
func applyXKompoxResources(c *corev1.Container, ext any) {
	if ext == nil {
		return
	}
	b, err := yaml.Marshal(ext)
	if err != nil {
		return
	}
	var x struct {
		Resources struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"resources"`
		Limits struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"limits"`
	}
	if err := yaml.Unmarshal(b, &x); err != nil {
		return
	}
	rr := corev1.ResourceRequirements{}
	if x.Resources.CPU != "" || x.Resources.Memory != "" {
		rr.Requests = corev1.ResourceList{}
	}
	if x.Resources.CPU != "" {
		if q, err := resource.ParseQuantity(x.Resources.CPU); err == nil {
			rr.Requests[corev1.ResourceCPU] = q
		}
	}
	if x.Resources.Memory != "" {
		if q, err := resource.ParseQuantity(x.Resources.Memory); err == nil {
			rr.Requests[corev1.ResourceMemory] = q
		}
	}
	if x.Limits.CPU != "" || x.Limits.Memory != "" {
		if rr.Limits == nil {
			rr.Limits = corev1.ResourceList{}
		}
	}
	if x.Limits.CPU != "" {
		if q, err := resource.ParseQuantity(x.Limits.CPU); err == nil {
			rr.Limits[corev1.ResourceCPU] = q
		}
	}
	if x.Limits.Memory != "" {
		if q, err := resource.ParseQuantity(x.Limits.Memory); err == nil {
			rr.Limits[corev1.ResourceMemory] = q
		}
	}
	if len(rr.Requests) > 0 || len(rr.Limits) > 0 {
		c.Resources = rr
	}
}

// bytesToQuantity converts bytes to a resource.Quantity, rounding up to Mi boundary.
func bytesToQuantity(b int64) resource.Quantity {
	if b <= 0 {
		// Return zero-value quantity (interpreted as 0) to let API server raise validation error if invalid.
		return resource.Quantity{}
	}
	const Mi = int64(1 << 20)
	if b%Mi != 0 {
		b = ((b / Mi) + 1) * Mi
	}
	return resource.MustParse(fmt.Sprintf("%dMi", b>>20))
}
