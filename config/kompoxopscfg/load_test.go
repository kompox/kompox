package kompoxopscfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kompoxops.yml")

	content := `
version: v1
service:
  name: ops
provider:
  name: k3s1
  driver: k3s
  settings:
    KUBECONFIG: ~/.kube/config
cluster:
  name: test-cluster
  existing: false
  ingress:
    domain: ops.kompox.dev
    controller: traefik
    namespace: traefik
    certEmail: admin@example.com
    certResolver: staging
  settings:
    NODE_COUNT: "3"
app:
  name: sample-app
  compose: ./docker-compose.yml
  ingress:
    certResolver: production
    rules:
      - name: http_80
        port: 80
        hosts:
          - app.example.com
  resources:
    cpu: 100m
    memory: 256Mi
  settings:
    REPLICAS: "2"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Version != "v1" {
		t.Errorf("expected version v1, got %s", cfg.Version)
	}
	if cfg.Service.Name != "ops" {
		t.Errorf("unexpected service name: %s", cfg.Service.Name)
	}
	if cfg.Provider.Name != "k3s1" || cfg.Provider.Driver != "k3s" {
		t.Errorf("unexpected provider: %+v", cfg.Provider)
	}
	if cfg.Cluster.Name != "test-cluster" || cfg.Cluster.Ingress.Domain != "ops.kompox.dev" {
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

func TestLoad_InvalidVolumeName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-volume.yml")

	content := `version: v1
service:
  name: ops
provider:
  name: k3s1
  driver: k3s
cluster:
  name: test-cluster
app:
  name: sample-app
  compose: ./docker-compose.yml
  volumes:
    - name: INVALID
      size: 1Gi
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp yaml: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "invalid volume name") {
		t.Fatalf("unexpected error: %v", err)
	}
}
