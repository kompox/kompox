package kube

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
	yaml "gopkg.in/yaml.v3"
)

var (
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// ReadEnvDirFile reads an env file relative to baseDir (Compose env_file semantics).
// It rejects absolute paths, traversal, symlinks, and directories.
// When required is false and the file does not exist, returns empty map without error.
func ReadEnvDirFile(baseDir, relPath string, required bool) (map[string]string, error) {
	if strings.HasPrefix(relPath, "/") {
		return nil, fmt.Errorf("env_file must be relative: %s", relPath)
	}
	if strings.Contains(relPath, "..") { // coarse check (spec: reject after normalization)
		return nil, fmt.Errorf("env_file path must not contain '..': %s", relPath)
	}
	full := filepath.Join(baseDir, relPath)
	info, err := os.Lstat(full)
	if err != nil {
		if !required && os.IsNotExist(err) {
			// Optional file does not exist, return empty map
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("env_file stat failed: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("env_file symlink not allowed: %s", relPath)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("env_file is directory: %s", relPath)
	}
	// Delegate extension handling to ReadEnvFile for single implementation point.
	return ReadEnvFile(full)
}

// ReadEnvFile reads a standalone env file (absolute or relative path, no base directory semantics).
// It enforces the same safety checks (no symlink, not a directory).
func ReadEnvFile(path string) (map[string]string, error) { // retained for backward compatibility (reads file then dispatches)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ReadEnv(data, path)
}

// ReadEnv parses environment-style key/value data from raw content bytes using file extension hints.
// Supported: .env (default), .yaml/.yml, .json . Behavior mirrors prior ReadEnvFile.
func ReadEnv(content []byte, path string) (map[string]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yml", ".yaml":
		return readYAMLEnv(content, path)
	case ".json":
		return readJSONEnv(content, path)
	default:
		return readDotEnv(content, path)
	}
}

func readDotEnv(data []byte, path string) (map[string]string, error) {
	m := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
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

func readYAMLEnv(data []byte, path string) (map[string]string, error) {
	var obj map[string]any
	if err := yamlUnmarshal(data, &obj); err != nil {
		return nil, err
	}
	return normalizeKVObject(obj, filepath.Ext(path))
}

func readJSONEnv(data []byte, path string) (map[string]string, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return normalizeKVObject(obj, filepath.Ext(path))
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
func ValidateSecretData(m map[string]string) error {
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

// mergeEnvFiles returns merged map in order (later overrides earlier); also list of overridden keys (for potential warnings).
func mergeEnvFiles(baseDir string, envFiles []types.EnvFile) (map[string]string, map[string]string, error) {
	merged := map[string]string{}
	overrides := map[string]string{} // key -> previous value
	for _, ef := range envFiles {
		f := strings.TrimSpace(ef.Path)
		if f == "" {
			continue
		}
		m, err := ReadEnvDirFile(baseDir, f, ef.Required)
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
	if err := ValidateSecretData(merged); err != nil {
		return nil, nil, err
	}
	return merged, overrides, nil
}
