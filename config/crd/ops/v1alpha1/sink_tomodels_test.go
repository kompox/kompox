package v1alpha1

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

// Mock repositories for testing
type mockWorkspaceRepo struct {
	workspaces []*model.Workspace
	createErr  error
}

func (m *mockWorkspaceRepo) Create(ctx context.Context, ws *model.Workspace) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Honor pre-set ID (FQN), only auto-generate when empty
	if ws.ID == "" {
		ws.ID = "ws-" + ws.Name
	}
	m.workspaces = append(m.workspaces, ws)
	return nil
}

func (m *mockWorkspaceRepo) List(ctx context.Context) ([]*model.Workspace, error) {
	return m.workspaces, nil
}

type mockProviderRepo struct {
	providers []*model.Provider
	createErr error
}

func (m *mockProviderRepo) Create(ctx context.Context, prv *model.Provider) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Honor pre-set ID (FQN), only auto-generate when empty
	if prv.ID == "" {
		prv.ID = "prv-" + prv.Name
	}
	m.providers = append(m.providers, prv)
	return nil
}

func (m *mockProviderRepo) List(ctx context.Context) ([]*model.Provider, error) {
	return m.providers, nil
}

type mockClusterRepo struct {
	clusters  []*model.Cluster
	createErr error
}

func (m *mockClusterRepo) Create(ctx context.Context, cls *model.Cluster) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Honor pre-set ID (FQN), only auto-generate when empty
	if cls.ID == "" {
		cls.ID = "cls-" + cls.Name
	}
	m.clusters = append(m.clusters, cls)
	return nil
}

func (m *mockClusterRepo) List(ctx context.Context) ([]*model.Cluster, error) {
	return m.clusters, nil
}

type mockAppRepo struct {
	apps      []*model.App
	createErr error
}

func (m *mockAppRepo) Create(ctx context.Context, app *model.App) error {
	if m.createErr != nil {
		return m.createErr
	}
	// Honor pre-set ID (FQN), only auto-generate when empty
	if app.ID == "" {
		app.ID = "app-" + app.Name
	}
	m.apps = append(m.apps, app)
	return nil
}

func (m *mockAppRepo) List(ctx context.Context) ([]*model.App, error) {
	return m.apps, nil
}

func TestSink_ToModels(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantErr     bool
		validate    func(t *testing.T, repos Repositories)
	}{
		{
			name: "empty sink - single workspace",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: empty-ws
  annotations:
    ops.kompox.dev/id: /ws/empty-ws
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				workspaces, _ := repos.Workspace.List(context.Background())
				if len(workspaces) != 1 {
					t.Errorf("expected 1 workspace, got %d", len(workspaces))
				}
				if workspaces[0].Name != "empty-ws" {
					t.Errorf("expected workspace name 'empty-ws', got %q", workspaces[0].Name)
				}
			},
		},
		{
			name: "workspace with provider",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
  annotations:
    ops.kompox.dev/id: /ws/test-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-prv
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv
spec:
  driver: aks
  settings:
    location: "eastus"
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				providers, _ := repos.Provider.List(context.Background())
				if len(providers) != 1 {
					t.Errorf("expected 1 provider, got %d", len(providers))
				}
				if providers[0].Name != "test-prv" {
					t.Errorf("expected provider name 'test-prv', got %q", providers[0].Name)
				}
				if providers[0].Driver != "aks" {
					t.Errorf("expected driver 'aks', got %q", providers[0].Driver)
				}
				// Expect Resource ID format for WorkspaceID
				if providers[0].WorkspaceID != "/ws/test-ws" {
					t.Errorf("expected workspace ID '/ws/test-ws', got %q", providers[0].WorkspaceID)
				}
			},
		},
		{
			name: "full hierarchy: workspace -> provider -> cluster -> app",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: my-ws
  annotations:
    ops.kompox.dev/id: /ws/my-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: my-prv
  annotations:
    ops.kompox.dev/id: /ws/my-ws/prv/my-prv
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: my-cls
  annotations:
    ops.kompox.dev/id: /ws/my-ws/prv/my-prv/cls/my-cls
spec:
  existing: true
  ingress:
    namespace: traefik
    controller: traefik
    domain: example.com
    certResolver: letsencrypt
    certEmail: admin@example.com
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: my-app
  annotations:
    ops.kompox.dev/id: /ws/my-ws/prv/my-prv/cls/my-cls/app/my-app
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				// Check workspace
				workspaces, _ := repos.Workspace.List(context.Background())
				if len(workspaces) != 1 {
					t.Fatalf("expected 1 workspace, got %d", len(workspaces))
				}

				// Check provider
				providers, _ := repos.Provider.List(context.Background())
				if len(providers) != 1 {
					t.Fatalf("expected 1 provider, got %d", len(providers))
				}
				// Expect Resource ID format for WorkspaceID (parent path)
				if providers[0].WorkspaceID != "/ws/my-ws" {
					t.Errorf("expected provider WorkspaceID '/ws/my-ws', got %q", providers[0].WorkspaceID)
				}

				// Check cluster
				clusters, _ := repos.Cluster.List(context.Background())
				if len(clusters) != 1 {
					t.Fatalf("expected 1 cluster, got %d", len(clusters))
				}
				// Expect Resource ID format for ProviderID (parent path)
				if clusters[0].ProviderID != "/ws/my-ws/prv/my-prv" {
					t.Errorf("expected cluster ProviderID '/ws/my-ws/prv/my-prv', got %q", clusters[0].ProviderID)
				}
				if clusters[0].Existing != true {
					t.Errorf("expected cluster Existing=true, got %v", clusters[0].Existing)
				}
				if clusters[0].Ingress == nil {
					t.Fatal("expected cluster to have Ingress config")
				}
				if clusters[0].Ingress.Domain != "example.com" {
					t.Errorf("expected ingress domain 'example.com', got %q", clusters[0].Ingress.Domain)
				}

				// Check app
				apps, _ := repos.App.List(context.Background())
				if len(apps) != 1 {
					t.Fatalf("expected 1 app, got %d", len(apps))
				}
				// Expect Resource ID format for ClusterID (parent path)
				if apps[0].ClusterID != "/ws/my-ws/prv/my-prv/cls/my-cls" {
					t.Errorf("expected app ClusterID '/ws/my-ws/prv/my-prv/cls/my-cls', got %q", apps[0].ClusterID)
				}
			},
		},
		{
			name: "cluster with certificates",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: cert-ws
  annotations:
    ops.kompox.dev/id: /ws/cert-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: cert-prv
  annotations:
    ops.kompox.dev/id: /ws/cert-ws/prv/cert-prv
spec:
  driver: aks
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: cert-cls
  annotations:
    ops.kompox.dev/id: /ws/cert-ws/prv/cert-prv/cls/cert-cls
spec:
  ingress:
    namespace: ingress
    controller: nginx
    certificates:
      - name: cert1
        source: keyvault
      - name: cert2
        source: file
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				clusters, _ := repos.Cluster.List(context.Background())
				if len(clusters) != 1 {
					t.Fatalf("expected 1 cluster, got %d", len(clusters))
				}
				if clusters[0].Ingress == nil {
					t.Fatal("expected cluster to have Ingress config")
				}
				if len(clusters[0].Ingress.Certificates) != 2 {
					t.Errorf("expected 2 certificates, got %d", len(clusters[0].Ingress.Certificates))
				}
				if len(clusters[0].Ingress.Certificates) > 0 && clusters[0].Ingress.Certificates[0].Name != "cert1" {
					t.Errorf("expected cert name 'cert1', got %q", clusters[0].Ingress.Certificates[0].Name)
				}
			},
		},
		{
			name: "missing parent workspace - should fail at NewSink validation",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: orphan-prv
  annotations:
    ops.kompox.dev/id: /ws/nonexistent-ws/prv/orphan-prv
spec:
  driver: aks
`,
			wantErr: true,
			validate: func(t *testing.T, repos Repositories) {
				// This test verifies that NewSink catches the validation error
			},
		},
		{
			name: "missing parent provider - should fail at NewSink validation",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
  annotations:
    ops.kompox.dev/id: /ws/test-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: orphan-cls
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/nonexistent-prv/cls/orphan-cls
spec:
  existing: false
`,
			wantErr: true,
			validate: func(t *testing.T, repos Repositories) {
				// This test verifies that NewSink catches the validation error
			},
		},
		{
			name: "missing parent cluster - should fail at NewSink validation",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: test-ws
  annotations:
    ops.kompox.dev/id: /ws/test-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: test-prv
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: orphan-app
  annotations:
    ops.kompox.dev/id: /ws/test-ws/prv/test-prv/cls/nonexistent-cls/app/orphan-app
`,
			wantErr: true,
			validate: func(t *testing.T, repos Repositories) {
				// This test verifies that NewSink catches the validation error
			},
		},
		{
			name: "app with full spec - compose, ingress, volumes, deployment, resources, settings",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: app-ws
  annotations:
    ops.kompox.dev/id: /ws/app-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: app-prv
  annotations:
    ops.kompox.dev/id: /ws/app-ws/prv/app-prv
spec:
  driver: aks
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: app-cls
  annotations:
    ops.kompox.dev/id: /ws/app-ws/prv/app-prv/cls/app-cls
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: full-app
  annotations:
    ops.kompox.dev/id: /ws/app-ws/prv/app-prv/cls/app-cls/app/full-app
spec:
  compose: |
    services:
      web:
        image: nginx:latest
        ports:
          - "80:80"
  ingress:
    certResolver: letsencrypt
    rules:
      - name: web
        port: 80
        hosts:
          - app.example.com
          - www.app.example.com
      - name: api
        port: 8080
        hosts:
          - api.example.com
  volumes:
    - name: data
      size: 10737418240
      options:
        sku: Premium_LRS
    - name: cache
      size: 5368709120
      options:
        sku: Standard_LRS
  deployment:
    pool: user
    zone: "1"
  resources:
    cpu: "1000m"
    memory: "1Gi"
  settings:
    replicas: "3"
    nodeSelector: "disktype=ssd"
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				apps, _ := repos.App.List(context.Background())
				if len(apps) != 1 {
					t.Fatalf("expected 1 app, got %d", len(apps))
				}
				app := apps[0]

				// Validate basic fields
				if app.Name != "full-app" {
					t.Errorf("expected app name 'full-app', got %q", app.Name)
				}
				if app.ClusterID != "/ws/app-ws/prv/app-prv/cls/app-cls" {
					t.Errorf("expected app ClusterID '/ws/app-ws/prv/app-prv/cls/app-cls', got %q", app.ClusterID)
				}

				// Validate compose
				if app.Compose == "" {
					t.Error("expected compose to be non-empty")
				}
				if app.Compose[:8] != "services" {
					t.Errorf("expected compose to start with 'services', got %q", app.Compose[:20])
				}

				// Validate ingress
				if app.Ingress.CertResolver != "letsencrypt" {
					t.Errorf("expected certResolver 'letsencrypt', got %q", app.Ingress.CertResolver)
				}
				if len(app.Ingress.Rules) != 2 {
					t.Fatalf("expected 2 ingress rules, got %d", len(app.Ingress.Rules))
				}
				if app.Ingress.Rules[0].Name != "web" {
					t.Errorf("expected first rule name 'web', got %q", app.Ingress.Rules[0].Name)
				}
				if app.Ingress.Rules[0].Port != 80 {
					t.Errorf("expected first rule port 80, got %d", app.Ingress.Rules[0].Port)
				}
				if len(app.Ingress.Rules[0].Hosts) != 2 {
					t.Errorf("expected 2 hosts for first rule, got %d", len(app.Ingress.Rules[0].Hosts))
				}
				if app.Ingress.Rules[1].Name != "api" {
					t.Errorf("expected second rule name 'api', got %q", app.Ingress.Rules[1].Name)
				}

				// Validate volumes
				if len(app.Volumes) != 2 {
					t.Fatalf("expected 2 volumes, got %d", len(app.Volumes))
				}
				if app.Volumes[0].Name != "data" {
					t.Errorf("expected first volume name 'data', got %q", app.Volumes[0].Name)
				}
				if app.Volumes[0].Size != 10737418240 {
					t.Errorf("expected first volume size 10737418240, got %d", app.Volumes[0].Size)
				}
				if app.Volumes[0].Options["sku"] != "Premium_LRS" {
					t.Errorf("expected first volume sku 'Premium_LRS', got %v", app.Volumes[0].Options["sku"])
				}
				if app.Volumes[1].Name != "cache" {
					t.Errorf("expected second volume name 'cache', got %q", app.Volumes[1].Name)
				}

				// Validate deployment
				if app.Deployment.Pool != "user" {
					t.Errorf("expected deployment pool 'user', got %q", app.Deployment.Pool)
				}
				if app.Deployment.Zone != "1" {
					t.Errorf("expected deployment zone '1', got %q", app.Deployment.Zone)
				}

				// Validate resources
				if len(app.Resources) != 2 {
					t.Errorf("expected 2 resources, got %d", len(app.Resources))
				}
				if app.Resources["cpu"] != "1000m" {
					t.Errorf("expected cpu '1000m', got %q", app.Resources["cpu"])
				}
				if app.Resources["memory"] != "1Gi" {
					t.Errorf("expected memory '1Gi', got %q", app.Resources["memory"])
				}

				// Validate settings
				if len(app.Settings) != 2 {
					t.Errorf("expected 2 settings, got %d", len(app.Settings))
				}
				if app.Settings["replicas"] != "3" {
					t.Errorf("expected replicas '3', got %q", app.Settings["replicas"])
				}
				if app.Settings["nodeSelector"] != "disktype=ssd" {
					t.Errorf("expected nodeSelector 'disktype=ssd', got %q", app.Settings["nodeSelector"])
				}
			},
		},
		{
			name: "app with minimal spec - only compose",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: min-ws
  annotations:
    ops.kompox.dev/id: /ws/min-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: min-prv
  annotations:
    ops.kompox.dev/id: /ws/min-ws/prv/min-prv
spec:
  driver: k3s
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: min-cls
  annotations:
    ops.kompox.dev/id: /ws/min-ws/prv/min-prv/cls/min-cls
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: min-app
  annotations:
    ops.kompox.dev/id: /ws/min-ws/prv/min-prv/cls/min-cls/app/min-app
spec:
  compose: "services:\n  web:\n    image: nginx\n"
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				apps, _ := repos.App.List(context.Background())
				if len(apps) != 1 {
					t.Fatalf("expected 1 app, got %d", len(apps))
				}
				app := apps[0]

				// Validate compose
				if app.Compose != "services:\n  web:\n    image: nginx\n" {
					t.Errorf("expected specific compose content, got %q", app.Compose)
				}

				// Validate empty/zero ingress (value type, not pointer)
				if app.Ingress.CertResolver != "" {
					t.Errorf("expected empty certResolver, got %q", app.Ingress.CertResolver)
				}
				if len(app.Ingress.Rules) != 0 {
					t.Errorf("expected no ingress rules, got %d", len(app.Ingress.Rules))
				}

				// Validate nil volumes
				if app.Volumes != nil {
					t.Errorf("expected nil volumes, got %v", app.Volumes)
				}

				// Validate empty deployment (value type, not pointer)
				if app.Deployment.Pool != "" {
					t.Errorf("expected empty deployment pool, got %q", app.Deployment.Pool)
				}
				if app.Deployment.Zone != "" {
					t.Errorf("expected empty deployment zone, got %q", app.Deployment.Zone)
				}

				// Validate nil resources and settings
				if app.Resources != nil {
					t.Errorf("expected nil resources, got %v", app.Resources)
				}
				if app.Settings != nil {
					t.Errorf("expected nil settings, got %v", app.Settings)
				}
			},
		},
		{
			name: "app with ingress but no rules",
			yamlContent: `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: ing-ws
  annotations:
    ops.kompox.dev/id: /ws/ing-ws
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Provider
metadata:
  name: ing-prv
  annotations:
    ops.kompox.dev/id: /ws/ing-ws/prv/ing-prv
spec:
  driver: aks
---
apiVersion: ops.kompox.dev/v1alpha1
kind: Cluster
metadata:
  name: ing-cls
  annotations:
    ops.kompox.dev/id: /ws/ing-ws/prv/ing-prv/cls/ing-cls
---
apiVersion: ops.kompox.dev/v1alpha1
kind: App
metadata:
  name: ing-app
  annotations:
    ops.kompox.dev/id: /ws/ing-ws/prv/ing-prv/cls/ing-cls/app/ing-app
spec:
  compose: "services: {}"
  ingress:
    certResolver: staging
`,
			wantErr: false,
			validate: func(t *testing.T, repos Repositories) {
				apps, _ := repos.App.List(context.Background())
				if len(apps) != 1 {
					t.Fatalf("expected 1 app, got %d", len(apps))
				}
				app := apps[0]

				// Validate ingress has certResolver but no rules
				if app.Ingress.CertResolver != "staging" {
					t.Errorf("expected certResolver 'staging', got %q", app.Ingress.CertResolver)
				}
				if len(app.Ingress.Rules) != 0 {
					t.Errorf("expected no ingress rules, got %d", len(app.Ingress.Rules))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary YAML file
			tmpDir := t.TempDir()
			yamlFile := filepath.Join(tmpDir, "test.yaml")
			if err := os.WriteFile(yamlFile, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Load documents
			loader := NewLoader()
			result, err := loader.Load(yamlFile)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			// Create sink from documents
			sink, err := NewSink(result.Documents)

			// If expecting error and NewSink already caught it, test passes
			if tt.wantErr && err != nil {
				return
			}

			if err != nil {
				t.Fatalf("NewSink() error = %v", err)
			}

			// Create mock repositories
			repos := Repositories{
				Workspace: &mockWorkspaceRepo{},
				Provider:  &mockProviderRepo{},
				Cluster:   &mockClusterRepo{},
				App:       &mockAppRepo{},
			}

			// Execute ToModels (empty kompoxAppFilePath for tests)
			err = sink.ToModels(context.Background(), repos, "")

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("Sink.ToModels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Run validation if no error expected
			if !tt.wantErr {
				tt.validate(t, repos)
			}
		})
	}
}

func TestSink_ToModels_RepositoryErrors(t *testing.T) {
	yamlContent := `apiVersion: ops.kompox.dev/v1alpha1
kind: Workspace
metadata:
  name: error-ws
  annotations:
    ops.kompox.dev/id: /ws/error-ws
`

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loader := NewLoader()
	result, err := loader.Load(yamlFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	sink, err := NewSink(result.Documents)
	if err != nil {
		t.Fatalf("NewSink() error = %v", err)
	}

	t.Run("workspace create error", func(t *testing.T) {
		repos := Repositories{
			Workspace: &mockWorkspaceRepo{createErr: context.DeadlineExceeded},
			Provider:  &mockProviderRepo{},
			Cluster:   &mockClusterRepo{},
			App:       &mockAppRepo{},
		}

		err := sink.ToModels(context.Background(), repos, "")
		if err == nil {
			t.Error("expected error when workspace creation fails")
		}
	})
}
