package aks

import (
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/kompox/kompox/domain/model"
)

func TestAgentPoolToNodePool(t *testing.T) {
	d := &driver{
		AzureLocation: "japaneast",
	}

	tests := []struct {
		name      string
		agentPool *armcontainerservice.AgentPool
		expected  *model.NodePool
	}{
		{
			name: "basic agent pool conversion",
			agentPool: &armcontainerservice.AgentPool{
				Name: to.Ptr("system"),
				Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
					Mode:              to.Ptr(armcontainerservice.AgentPoolModeSystem),
					VMSize:            to.Ptr("Standard_D2s_v3"),
					OSDiskType:        to.Ptr(armcontainerservice.OSDiskTypeManaged),
					OSDiskSizeGB:      to.Ptr(int32(128)),
					ScaleSetPriority:  to.Ptr(armcontainerservice.ScaleSetPriorityRegular),
					AvailabilityZones: []*string{to.Ptr("1"), to.Ptr("2")},
					EnableAutoScaling: to.Ptr(true),
					MinCount:          to.Ptr(int32(1)),
					MaxCount:          to.Ptr(int32(3)),
					Count:             to.Ptr(int32(2)),
					ProvisioningState: to.Ptr("Succeeded"),
				},
			},
			expected: &model.NodePool{
				Name:          to.Ptr("system"),
				ProviderName:  to.Ptr("system"),
				Mode:          to.Ptr("system"),
				InstanceType:  to.Ptr("Standard_D2s_v3"),
				OSDiskType:    to.Ptr("Managed"),
				OSDiskSizeGiB: to.Ptr(128),
				Priority:      to.Ptr("regular"),
				Zones:         &[]string{"japaneast-1", "japaneast-2"},
				Autoscaling: &model.NodePoolAutoscaling{
					Enabled: true,
					Min:     1,
					Max:     3,
				},
			},
		},
		{
			name: "user pool with no autoscaling",
			agentPool: &armcontainerservice.AgentPool{
				Name: to.Ptr("user"),
				Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
					Mode:              to.Ptr(armcontainerservice.AgentPoolModeUser),
					VMSize:            to.Ptr("Standard_D4s_v3"),
					EnableAutoScaling: to.Ptr(false),
					Count:             to.Ptr(int32(5)),
				},
			},
			expected: &model.NodePool{
				Name:         to.Ptr("user"),
				ProviderName: to.Ptr("user"),
				Mode:         to.Ptr("user"),
				InstanceType: to.Ptr("Standard_D4s_v3"),
				Autoscaling: &model.NodePoolAutoscaling{
					Enabled: false,
					Desired: to.Ptr(5),
				},
			},
		},
		{
			name: "pool with labels",
			agentPool: &armcontainerservice.AgentPool{
				Name: to.Ptr("compute"),
				Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
					Mode:   to.Ptr(armcontainerservice.AgentPoolModeUser),
					VMSize: to.Ptr("Standard_E4s_v3"),
					NodeLabels: map[string]*string{
						"workload": to.Ptr("compute"),
						"tier":     to.Ptr("production"),
					},
				},
			},
			expected: &model.NodePool{
				Name:         to.Ptr("compute"),
				ProviderName: to.Ptr("compute"),
				Mode:         to.Ptr("user"),
				InstanceType: to.Ptr("Standard_E4s_v3"),
				Labels: &map[string]string{
					"workload": "compute",
					"tier":     "production",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.agentPoolToNodePool(tt.agentPool)

			// Verify key fields
			if result.Name == nil || *result.Name != *tt.expected.Name {
				t.Errorf("Name: got %v, want %v", result.Name, tt.expected.Name)
			}
			if result.Mode == nil || *result.Mode != *tt.expected.Mode {
				t.Errorf("Mode: got %v, want %v", result.Mode, tt.expected.Mode)
			}
			if tt.expected.InstanceType != nil {
				if result.InstanceType == nil || *result.InstanceType != *tt.expected.InstanceType {
					t.Errorf("InstanceType: got %v, want %v", result.InstanceType, tt.expected.InstanceType)
				}
			}
			if tt.expected.Zones != nil {
				if result.Zones == nil {
					t.Errorf("Zones: got nil, want %v", *tt.expected.Zones)
				} else if len(*result.Zones) != len(*tt.expected.Zones) {
					t.Errorf("Zones length: got %d, want %d", len(*result.Zones), len(*tt.expected.Zones))
				} else {
					for i, z := range *tt.expected.Zones {
						if (*result.Zones)[i] != z {
							t.Errorf("Zone[%d]: got %s, want %s", i, (*result.Zones)[i], z)
						}
					}
				}
			}
			if tt.expected.Autoscaling != nil {
				if result.Autoscaling == nil {
					t.Errorf("Autoscaling: got nil, want %+v", *tt.expected.Autoscaling)
				} else {
					if result.Autoscaling.Enabled != tt.expected.Autoscaling.Enabled {
						t.Errorf("Autoscaling.Enabled: got %v, want %v", result.Autoscaling.Enabled, tt.expected.Autoscaling.Enabled)
					}
					if tt.expected.Autoscaling.Enabled {
						if result.Autoscaling.Min != tt.expected.Autoscaling.Min {
							t.Errorf("Autoscaling.Min: got %d, want %d", result.Autoscaling.Min, tt.expected.Autoscaling.Min)
						}
						if result.Autoscaling.Max != tt.expected.Autoscaling.Max {
							t.Errorf("Autoscaling.Max: got %d, want %d", result.Autoscaling.Max, tt.expected.Autoscaling.Max)
						}
					}
				}
			}
		})
	}
}

func TestValidateImmutableFields(t *testing.T) {
	d := &driver{
		AzureLocation: "japaneast",
	}

	existing := &armcontainerservice.AgentPool{
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
			Mode:              to.Ptr(armcontainerservice.AgentPoolModeSystem),
			VMSize:            to.Ptr("Standard_D2s_v3"),
			OSDiskType:        to.Ptr(armcontainerservice.OSDiskTypeManaged),
			OSDiskSizeGB:      to.Ptr(int32(128)),
			ScaleSetPriority:  to.Ptr(armcontainerservice.ScaleSetPriorityRegular),
			AvailabilityZones: []*string{to.Ptr("1"), to.Ptr("2")},
		},
	}

	tests := []struct {
		name        string
		update      model.NodePool
		expectError bool
		errorMsg    string
	}{
		{
			name: "no immutable fields changed",
			update: model.NodePool{
				Labels: &map[string]string{"new": "label"},
			},
			expectError: false,
		},
		{
			name: "mode changed (immutable)",
			update: model.NodePool{
				Mode: to.Ptr("user"),
			},
			expectError: true,
			errorMsg:    "Mode is immutable",
		},
		{
			name: "instance type changed (immutable)",
			update: model.NodePool{
				InstanceType: to.Ptr("Standard_D4s_v3"),
			},
			expectError: true,
			errorMsg:    "InstanceType is immutable",
		},
		{
			name: "os disk type changed (immutable)",
			update: model.NodePool{
				OSDiskType: to.Ptr("Ephemeral"),
			},
			expectError: true,
			errorMsg:    "OSDiskType is immutable",
		},
		{
			name: "os disk size changed (immutable)",
			update: model.NodePool{
				OSDiskSizeGiB: to.Ptr(256),
			},
			expectError: true,
			errorMsg:    "OSDiskSizeGiB is immutable",
		},
		{
			name: "priority changed (immutable)",
			update: model.NodePool{
				Priority: to.Ptr("spot"),
			},
			expectError: true,
			errorMsg:    "Priority is immutable",
		},
		{
			name: "zones changed (immutable)",
			update: model.NodePool{
				Zones: &[]string{"japaneast-1", "japaneast-2", "japaneast-3"},
			},
			expectError: true,
			errorMsg:    "Zones are immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.validateImmutableFields(tt.update, existing)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestNormalizeZones(t *testing.T) {
	d := &driver{
		AzureLocation: "japaneast",
	}

	tests := []struct {
		name           string
		aksZone        string
		expectedKompox string
		kompoxZone     string
		expectedAks    string
	}{
		{
			name:           "convert AKS zone 1",
			aksZone:        "1",
			expectedKompox: "japaneast-1",
			kompoxZone:     "japaneast-1",
			expectedAks:    "1",
		},
		{
			name:           "convert AKS zone 2",
			aksZone:        "2",
			expectedKompox: "japaneast-2",
			kompoxZone:     "japaneast-2",
			expectedAks:    "2",
		},
		{
			name:           "already in Kompox format",
			aksZone:        "japaneast-1",
			expectedKompox: "japaneast-1",
			kompoxZone:     "japaneast-1",
			expectedAks:    "1",
		},
		{
			name:           "already in AKS format",
			aksZone:        "3",
			expectedKompox: "japaneast-3",
			kompoxZone:     "3",
			expectedAks:    "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test AKS to Kompox
			result := d.normalizeAksZoneToKompox(tt.aksZone)
			if result != tt.expectedKompox {
				t.Errorf("normalizeAksZoneToKompox(%q): got %q, want %q", tt.aksZone, result, tt.expectedKompox)
			}

			// Test Kompox to AKS
			result = d.normalizeKompoxZoneToAks(tt.kompoxZone)
			if result != tt.expectedAks {
				t.Errorf("normalizeKompoxZoneToAks(%q): got %q, want %q", tt.kompoxZone, result, tt.expectedAks)
			}
		})
	}
}
