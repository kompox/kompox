package kompoxopscfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a YAML file from the given path and returns a deserialized Root.
// It performs no validation beyond YAML decoding; validation is handled elsewhere.
func Load(path string) (*Root, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var cfg Root
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &cfg, nil
}
