package kompoxopscfg

import (
	"testing"

	"github.com/kompox/kompox/domain/model"
)

func TestRoot_ToModels(t *testing.T) {
	tests := []struct {
		name string
		root Root
		want struct {
			workspaceName string
			providerName  string
			clusterName   string
			appName       string
			volumesCount  int
			volumeOptions map[string]any
		}
	}{
		{
			name: "basic conversion with volume options",
			root: Root{
				Version: "v1",
				Workspace: Workspace{
					Name: "test-workspace",
				},
				Provider: Provider{
					Name:   "test-provider",
					Driver: "aks",
					Settings: map[string]string{
						"AZURE_LOCATION": "japaneast",
					},
				},
				Cluster: Cluster{
					Name:     "test-cluster",
					Existing: false,
					Ingress: ClusterIngress{
						Namespace:    "traefik",
						Controller:   "traefik",
						Domain:       "example.com",
						CertResolver: "staging",
						CertEmail:    "admin@example.com",
					},
					Settings: map[string]string{
						"NODE_COUNT": "3",
					},
				},
				App: App{
					Name:    "test-app",
					Compose: "workspaces:\n  web:\n    image: nginx",
					Ingress: AppIngress{
						CertResolver: "production",
						Rules: []AppIngressRule{
							{
								Name:  "web",
								Port:  80,
								Hosts: []string{"www.example.com"},
							},
						},
					},
					Volumes: []AppVolume{
						{
							Name: "data",
							Size: "32Gi",
							Options: map[string]any{
								"sku":  "PremiumV2_LRS",
								"iops": 3000,
								"mbps": 125,
							},
						},
					},
					Deployment: AppDeployment{
						Pool: "user",
						Zone: "2",
					},
					Resources: map[string]string{
						"cpu":    "100m",
						"memory": "256Mi",
					},
					Settings: map[string]string{
						"REPLICAS": "2",
					},
				},
			},
			want: struct {
				workspaceName string
				providerName  string
				clusterName   string
				appName       string
				volumesCount  int
				volumeOptions map[string]any
			}{
				workspaceName: "test-workspace",
				providerName:  "test-provider",
				clusterName:   "test-cluster",
				appName:       "test-app",
				volumesCount:  1,
				volumeOptions: map[string]any{
					"sku":  "PremiumV2_LRS",
					"iops": 3000,
					"mbps": 125,
				},
			},
		},
		{
			name: "conversion without volume options",
			root: Root{
				Version: "v1",
				Workspace: Workspace{
					Name: "simple-workspace",
				},
				Provider: Provider{
					Name:   "simple-provider",
					Driver: "k3s",
				},
				Cluster: Cluster{
					Name:     "simple-cluster",
					Existing: true,
				},
				App: App{
					Name:    "simple-app",
					Compose: "workspaces:\n  app:\n    image: hello-world",
					Volumes: []AppVolume{
						{
							Name: "simple-volume",
							Size: "10Gi",
							// No Options field
						},
					},
				},
			},
			want: struct {
				workspaceName string
				providerName  string
				clusterName   string
				appName       string
				volumesCount  int
				volumeOptions map[string]any
			}{
				workspaceName: "simple-workspace",
				providerName:  "simple-provider",
				clusterName:   "simple-cluster",
				appName:       "simple-app",
				volumesCount:  1,
				volumeOptions: nil, // No options specified
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace, provider, cluster, app, err := tt.root.ToModels("")
			if err != nil {
				t.Fatalf("ToModels() error = %v", err)
			}

			// Verify workspace
			if workspace.Name != tt.want.workspaceName {
				t.Errorf("workspace.Name = %v, want %v", workspace.Name, tt.want.workspaceName)
			}
			if workspace.ID == "" {
				t.Error("workspace.ID should not be empty")
			}

			// Verify provider
			if provider.Name != tt.want.providerName {
				t.Errorf("provider.Name = %v, want %v", provider.Name, tt.want.providerName)
			}
			if provider.WorkspaceID != workspace.ID {
				t.Errorf("provider.WorkspaceID = %v, want %v", provider.WorkspaceID, workspace.ID)
			}

			// Verify cluster
			if cluster.Name != tt.want.clusterName {
				t.Errorf("cluster.Name = %v, want %v", cluster.Name, tt.want.clusterName)
			}
			if cluster.ProviderID != provider.ID {
				t.Errorf("cluster.ProviderID = %v, want %v", cluster.ProviderID, provider.ID)
			}

			// Verify app
			if app.Name != tt.want.appName {
				t.Errorf("app.Name = %v, want %v", app.Name, tt.want.appName)
			}
			if app.ClusterID != cluster.ID {
				t.Errorf("app.ClusterID = %v, want %v", app.ClusterID, cluster.ID)
			}

			// Verify volumes
			if len(app.Volumes) != tt.want.volumesCount {
				t.Errorf("len(app.Volumes) = %v, want %v", len(app.Volumes), tt.want.volumesCount)
			}

			if tt.want.volumesCount > 0 {
				volume := app.Volumes[0]
				if volume.Size <= 0 {
					t.Error("volume.Size should be positive")
				}

				// Check volume options
				if tt.want.volumeOptions == nil {
					if volume.Options != nil {
						t.Errorf("volume.Options = %v, want nil", volume.Options)
					}
				} else {
					if volume.Options == nil {
						t.Error("volume.Options should not be nil")
					} else {
						for key, expectedValue := range tt.want.volumeOptions {
							if actualValue, exists := volume.Options[key]; !exists {
								t.Errorf("volume.Options[%q] not found", key)
							} else if actualValue != expectedValue {
								t.Errorf("volume.Options[%q] = %v, want %v", key, actualValue, expectedValue)
							}
						}
					}
				}
			}

			// Verify timestamps are set
			if workspace.CreatedAt.IsZero() {
				t.Error("workspace.CreatedAt should be set")
			}
			if provider.CreatedAt.IsZero() {
				t.Error("provider.CreatedAt should be set")
			}
			if cluster.CreatedAt.IsZero() {
				t.Error("cluster.CreatedAt should be set")
			}
			if app.CreatedAt.IsZero() {
				t.Error("app.CreatedAt should be set")
			}
		})
	}
}

func TestToModelVolumes(t *testing.T) {
	tests := []struct {
		name    string
		volumes []AppVolume
		want    []model.AppVolume
	}{
		{
			name:    "empty volumes",
			volumes: nil,
			want:    nil,
		},
		{
			name: "volumes with options",
			volumes: []AppVolume{
				{
					Name: "data",
					Size: "32Gi",
					Options: map[string]any{
						"sku":  "PremiumV2_LRS",
						"iops": 3000,
						"mbps": 125,
					},
				},
				{
					Name: "cache",
					Size: "8Gi",
					Options: map[string]any{
						"sku": "Premium_LRS",
					},
				},
			},
			want: []model.AppVolume{
				{
					Name: "data",
					Size: 32 * 1024 * 1024 * 1024, // 32Gi in bytes
					Options: map[string]any{
						"sku":  "PremiumV2_LRS",
						"iops": 3000,
						"mbps": 125,
					},
				},
				{
					Name: "cache",
					Size: 8 * 1024 * 1024 * 1024, // 8Gi in bytes
					Options: map[string]any{
						"sku": "Premium_LRS",
					},
				},
			},
		},
		{
			name: "volumes without options",
			volumes: []AppVolume{
				{
					Name: "simple",
					Size: "1Gi",
				},
			},
			want: []model.AppVolume{
				{
					Name:    "simple",
					Size:    1024 * 1024 * 1024, // 1Gi in bytes
					Options: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toModelVolumes(tt.volumes)

			if len(got) != len(tt.want) {
				t.Errorf("toModelVolumes() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i, volume := range got {
				expected := tt.want[i]

				if volume.Name != expected.Name {
					t.Errorf("volume[%d].Name = %v, want %v", i, volume.Name, expected.Name)
				}

				if volume.Size != expected.Size {
					t.Errorf("volume[%d].Size = %v, want %v", i, volume.Size, expected.Size)
				}

				// Compare options
				if expected.Options == nil {
					if volume.Options != nil {
						t.Errorf("volume[%d].Options = %v, want nil", i, volume.Options)
					}
				} else {
					if volume.Options == nil {
						t.Errorf("volume[%d].Options = nil, want %v", i, expected.Options)
					} else {
						for key, expectedValue := range expected.Options {
							if actualValue, exists := volume.Options[key]; !exists {
								t.Errorf("volume[%d].Options[%q] not found", i, key)
							} else if actualValue != expectedValue {
								t.Errorf("volume[%d].Options[%q] = %v, want %v", i, key, actualValue, expectedValue)
							}
						}
					}
				}
			}
		})
	}
}

func TestToModelVolumes_InvalidSize(t *testing.T) {
	volumes := []AppVolume{
		{
			Name: "invalid",
			Size: "invalid-size",
		},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("toModelVolumes() should panic for invalid size")
		}
	}()

	toModelVolumes(volumes)
}
