package cfgops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kompoxops.yml")

	content := `
version: 1
service:
  name: ops
  domain: ops.kompox.dev
cluster:
  name: test-cluster
  auth:
    type: kubectl
  ingress:
    controller: traefik
    namespace: traefik
  provider: k3s
app:
  name: sample-app
  compose: ./docker-compose.yml
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Service.Name != "ops" || cfg.Service.Domain != "ops.kompox.dev" {
		t.Errorf("unexpected service: %+v", cfg.Service)
	}
	if cfg.Cluster.Name != "test-cluster" || cfg.Cluster.Provider != "k3s" {
		t.Errorf("unexpected cluster: %+v", cfg.Cluster)
	}
	if cfg.Cluster.Ingress.Controller != "traefik" || cfg.Cluster.Ingress.Namespace != "traefik" {
		t.Errorf("unexpected ingress: %+v", cfg.Cluster.Ingress)
	}
	if cfg.App.Name != "sample-app" || cfg.App.Compose != "./docker-compose.yml" {
		t.Errorf("unexpected app: %+v", cfg.App)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load("/path/does/not/exist.yml"); err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")

	// invalid YAML (missing closing bracket)
	bad := "version: [1,2\n"
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for invalid YAML, got nil")
	}
}
