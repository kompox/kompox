package kube

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/kompox/kompox/internal/naming"
	yaml "gopkg.in/yaml.v3"
)

var (
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// readEnvFile detects format by extension and dispatches.
func readEnvFile(baseDir, relPath string) (map[string]string, error) {
	if strings.HasPrefix(relPath, "/") {
		return nil, fmt.Errorf("env_file must be relative: %s", relPath)
	}
	if strings.Contains(relPath, "..") { // coarse check (spec: reject after normalization)
		return nil, fmt.Errorf("env_file path must not contain '..': %s", relPath)
	}
	full := filepath.Join(baseDir, relPath)
	info, err := os.Lstat(full)
	if err != nil {
		return nil, fmt.Errorf("env_file stat failed: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("env_file symlink not allowed: %s", relPath)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("env_file is directory: %s", relPath)
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".yml", ".yaml":
		return readYAMLEnv(full)
	case ".json":
		return readJSONEnv(full)
	default:
		return readDotEnv(full)
	}
}

func readDotEnv(full string) (map[string]string, error) {
	f, err := os.Open(full)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := map[string]string{}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		i := strings.IndexByte(line, '=')
		if i < 0 {
			return nil, fmt.Errorf(".env line %d missing '='", lineNo)
		}
		key := strings.TrimSpace(line[:i])
		if !envKeyRe.MatchString(key) {
			return nil, fmt.Errorf("invalid env key %q at line %d", key, lineNo)
		}
		rawVal := line[i+1:]
		// Unquoted remove a single leading space per spec.
		if !(strings.HasPrefix(rawVal, "\"") || strings.HasPrefix(rawVal, "'")) {
			rawVal = strings.TrimPrefix(rawVal, " ")
			m[key] = rawVal
			continue
		}
		if strings.HasPrefix(rawVal, "\"") { // double quotes
			if !strings.HasSuffix(rawVal, "\"") || len(rawVal) < 2 {
				return nil, fmt.Errorf("unterminated double quote line %d", lineNo)
			}
			body := rawVal[1 : len(rawVal)-1]
			// escape sequences \ \" \n \r \t
			var b strings.Builder
			for i := 0; i < len(body); i++ {
				ch := body[i]
				if ch == '\\' {
					if i+1 >= len(body) {
						return nil, fmt.Errorf("dangling escape line %d", lineNo)
					}
					esc := body[i+1]
					i++
					switch esc {
					case '\\', '"':
						b.WriteByte(esc)
					case 'n':
						b.WriteByte('\n')
					case 'r':
						b.WriteByte('\r')
					case 't':
						b.WriteByte('\t')
					default:
						return nil, fmt.Errorf("unsupported escape \\%c line %d", esc, lineNo)
					}
				} else {
					b.WriteByte(ch)
				}
			}
			m[key] = b.String()
		} else if strings.HasPrefix(rawVal, "'") { // single quotes
			if !strings.HasSuffix(rawVal, "'") || len(rawVal) < 2 {
				return nil, fmt.Errorf("unterminated single quote line %d", lineNo)
			}
			m[key] = rawVal[1 : len(rawVal)-1]
		} else {
			return nil, fmt.Errorf("invalid quoting line %d", lineNo)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

func readYAMLEnv(full string) (map[string]string, error) {
	// Use yaml.v3
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := yamlUnmarshal(data, &obj); err != nil {
		return nil, err
	}
	return normalizeKVObject(obj, filepath.Ext(full))
}

func readJSONEnv(full string) (map[string]string, error) {
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return normalizeKVObject(obj, filepath.Ext(full))
}

// normalizeKVObject validates object values and coerces to string.
func normalizeKVObject(obj map[string]any, ext string) (map[string]string, error) {
	if obj == nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range obj {
		switch vv := v.(type) {
		case nil:
			return nil, fmt.Errorf("key %s has null value (%s not allowed)", k, ext)
		case string:
			out[k] = vv
		case bool:
			if vv {
				out[k] = "true"
			} else {
				out[k] = "false"
			}
		case int, int64, int32, float64, float32:
			out[k] = fmt.Sprint(vv)
		default:
			return nil, fmt.Errorf("key %s has unsupported value type %T", k, v)
		}
	}
	return out, nil
}

// yamlUnmarshal is wrapped for test override if needed.
var yamlUnmarshal = func(b []byte, v any) error {
	return yaml.Unmarshal(b, v)
}

// Validate accumulated secret size and value runes.
func validateSecretData(m map[string]string) error {
	var total int
	for k, v := range m {
		if !envKeyRe.MatchString(k) {
			return fmt.Errorf("invalid env key %s", k)
		}
		if err := rejectControl(v); err != nil {
			return fmt.Errorf("value for %s: %w", k, err)
		}
		total += len(k) + len(v)
		if total > 1_000_000 {
			return fmt.Errorf("secret data exceeds 1000000 bytes")
		}
	}
	return nil
}

func rejectControl(s string) error {
	for _, r := range s {
		if r == 0 {
			return errors.New("contains NUL byte")
		}
		if (r >= 0x01 && r <= 0x08) || r == 0x0B || r == 0x0C || (r >= 0x0E && r <= 0x1F) || r == 0x7F {
			return fmt.Errorf("contains control char 0x%02X", r)
		}
	}
	return nil
}

// computeSecretHash returns base36 SHA256 prefix length 6 using naming.ShortHash logic.
func computeSecretHash(kv map[string]string) string {
	if len(kv) == 0 {
		return naming.ShortHash("", 6)
	}
	var keys []string
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(kv[k])
		b.WriteByte(0)
	}
	return naming.ShortHash(b.String(), 6)
}

// mergeEnvFiles returns merged map in order (later overrides earlier); also list of overridden keys (for potential warnings).
func mergeEnvFiles(baseDir string, files []string) (map[string]string, map[string]string, error) {
	merged := map[string]string{}
	overrides := map[string]string{} // key -> previous value
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		m, err := readEnvFile(baseDir, f)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range m {
			if prev, ok := merged[k]; ok && prev != v {
				overrides[k] = prev
			}
			merged[k] = v
		}
	}
	if err := validateSecretData(merged); err != nil {
		return nil, nil, err
	}
	return merged, overrides, nil
}

// Exposed helper for converter (minimal surface).
func buildComposeSecret(baseDir, appName, component, container string, envFiles []types.EnvFile, envOverrides map[string]*string, labels map[string]string, namespace string) (*corev1.Secret, string, error) {
	if len(envFiles) == 0 {
		return nil, "", nil
	}
	// Collect relative paths list.
	var files []string
	for _, ef := range envFiles {
		files = append(files, ef.Path)
	}
	kv, _, err := mergeEnvFiles(baseDir, files)
	if err != nil {
		return nil, "", err
	}
	// Remove keys overridden by service.Environment (those env overrides are not stored in Secret per spec)
	for k := range envOverrides {
		delete(kv, k)
	}
	hash := computeSecretHash(kv)
	data := map[string][]byte{}
	for k, v := range kv {
		data[k] = []byte(v)
	}
	name := fmt.Sprintf("%s-%s-%s", appName, component, container)
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels, Annotations: map[string]string{AnnotationK4xComposeSecretHash: hash}}, Type: corev1.SecretTypeOpaque, Data: data}
	return sec, hash, nil
}
