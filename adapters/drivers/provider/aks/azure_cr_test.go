package aks

import (
	"testing"

	"github.com/kompox/kompox/domain/model"
)

func TestParseAzureContainerRegistryID(t *testing.T) {
	tests := []struct {
		name        string
		resourceID  string
		wantName    string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid ACR resource ID",
			resourceID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myRG/providers/Microsoft.ContainerRegistry/registries/myregistry",
			wantName:   "myregistry",
			wantErr:    false,
		},
		{
			name:        "empty resource ID",
			resourceID:  "",
			wantErr:     true,
			errContains: "parse Azure Container Registry resource ID",
		},
		{
			name:        "invalid resource type - DNS zone",
			resourceID:  "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myRG/providers/Microsoft.Network/dnszones/example.com",
			wantErr:     true,
			errContains: "invalid resource type for Container Registry",
		},
		{
			name:        "invalid resource type - storage account",
			resourceID:  "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myRG/providers/Microsoft.Storage/storageAccounts/mystorageacct",
			wantErr:     true,
			errContains: "invalid resource type for Container Registry",
		},
		{
			name:        "malformed resource ID",
			resourceID:  "not-a-valid-resource-id",
			wantErr:     true,
			errContains: "parse Azure Container Registry resource ID",
		},
		{
			name:        "incomplete resource ID - missing name",
			resourceID:  "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myRG/providers/Microsoft.ContainerRegistry/registries/",
			wantErr:     true,
			errContains: "parse Azure Container Registry resource ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseAzureContainerRegistryID(tt.resourceID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseAzureContainerRegistryID() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("parseAzureContainerRegistryID() error = %q, want substring %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("parseAzureContainerRegistryID() unexpected error = %v", err)
				return
			}
			if info.Name != tt.wantName {
				t.Errorf("parseAzureContainerRegistryID() name = %q, want %q", info.Name, tt.wantName)
			}
			if info.ResourceID != tt.resourceID {
				t.Errorf("parseAzureContainerRegistryID() resourceID = %q, want %q", info.ResourceID, tt.resourceID)
			}
		})
	}
}

func TestCollectAzureContainerRegistryIDs(t *testing.T) {
	validID1 := "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/rg1/providers/Microsoft.ContainerRegistry/registries/registry1"
	validID2 := "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/rg2/providers/Microsoft.ContainerRegistry/registries/registry2"

	tests := []struct {
		name      string
		cluster   *model.Cluster
		wantCount int
		wantErr   bool
	}{
		{
			name:      "nil cluster",
			cluster:   nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "nil settings",
			cluster:   &model.Cluster{},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "empty setting",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: "",
				},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "single valid ACR ID",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: validID1,
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple ACR IDs - comma separated",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: validID1 + "," + validID2,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "multiple ACR IDs - space separated",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: validID1 + " " + validID2,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "multiple ACR IDs - mixed separators",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: validID1 + ", " + validID2,
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "invalid resource type in list",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/rg/providers/Microsoft.Network/dnszones/example.com",
				},
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "whitespace only",
			cluster: &model.Cluster{
				Settings: map[string]string{
					settingAzureAKSContainerRegistryResourceIDs: "   \t\n  ",
				},
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &driver{} // minimal driver for method call
			registries, err := d.collectAzureContainerRegistryIDs(tt.cluster)
			if tt.wantErr {
				if err == nil {
					t.Errorf("collectAzureContainerRegistryIDs() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("collectAzureContainerRegistryIDs() unexpected error = %v", err)
				return
			}
			if len(registries) != tt.wantCount {
				t.Errorf("collectAzureContainerRegistryIDs() got %d registries, want %d", len(registries), tt.wantCount)
			}
		})
	}
}

// contains is a simple substring check helper.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || anyIndex(s, substr) >= 0)
}

func anyIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
