package kompoxopscfg

import (
	"strings"
	"testing"
)

func TestRootValidate_VolumeNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		root    Root
		wantErr string
	}{
		{
			name: "no volumes",
			root: Root{
				App: App{},
			},
		},
		{
			name: "valid volume",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "data", Size: "1Gi"}},
				},
			},
		},
		{
			name: "invalid dns label",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "INVALID", Size: "1Gi"}},
				},
			},
			wantErr: "invalid volume name",
		},
		{
			name: "duplicate name",
			root: Root{
				App: App{
					Volumes: []AppVolume{
						{Name: "data", Size: "1Gi"},
						{Name: "data", Size: "2Gi"},
					},
				},
			},
			wantErr: "duplicate volume name",
		},
		{
			name: "valid type disk",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "data", Size: "1Gi", Type: "disk"}},
				},
			},
		},
		{
			name: "valid type files",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "data", Size: "1Gi", Type: "files"}},
				},
			},
		},
		{
			name: "empty type defaults to disk",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "data", Size: "1Gi", Type: ""}},
				},
			},
		},
		{
			name: "invalid type",
			root: Root{
				App: App{
					Volumes: []AppVolume{{Name: "data", Size: "1Gi", Type: "invalid"}},
				},
			},
			wantErr: "invalid type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.root.Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Validate() error = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("Validate() error = nil, want contains %q", tt.wantErr)
			case tt.wantErr != "" && err != nil && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("Validate() error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func TestRootValidate_Deployment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		root    Root
		wantErr string
	}{
		{
			name: "deployment with pool only",
			root: Root{App: App{Deployment: AppDeployment{Pool: "user"}}},
		},
		{
			name: "deployment with pools only",
			root: Root{App: App{Deployment: AppDeployment{Pools: []string{"npuser1", "npuser2"}}}},
		},
		{
			name: "deployment with zone only",
			root: Root{App: App{Deployment: AppDeployment{Zone: "japaneast-1"}}},
		},
		{
			name: "deployment with zones only",
			root: Root{App: App{Deployment: AppDeployment{Zones: []string{"japaneast-1", "japaneast-2"}}}},
		},
		{
			name:    "deployment with pool and pools",
			root:    Root{App: App{Deployment: AppDeployment{Pool: "user", Pools: []string{"npuser1"}}}},
			wantErr: "deployment.pool and deployment.pools cannot be specified together",
		},
		{
			name:    "deployment with zone and zones",
			root:    Root{App: App{Deployment: AppDeployment{Zone: "japaneast-1", Zones: []string{"japaneast-1"}}}},
			wantErr: "deployment.zone and deployment.zones cannot be specified together",
		},
		{
			name:    "deployment selectors reserved",
			root:    Root{App: App{Deployment: AppDeployment{Selectors: map[string]string{"disktype": "ssd"}}}},
			wantErr: "deployment.selectors is reserved and not supported yet",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.root.Validate()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Validate() error = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("Validate() error = nil, want contains %q", tt.wantErr)
			case tt.wantErr != "" && err != nil && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("Validate() error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}
