package kube

import (
	"context"
	"strings"
	"testing"

	"github.com/yaegashi/kompoxops/domain/model"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// mockDriver provides minimal VolumeClass implementation for testing
type mockDriver struct{}

func (m *mockDriver) ID() string                                                         { return "test" }
func (m *mockDriver) ServiceName() string                                                { return "testsvc" }
func (m *mockDriver) ProviderName() string                                               { return "testprv" }
func (m *mockDriver) ClusterProvision(ctx context.Context, cluster *model.Cluster) error { return nil }
func (m *mockDriver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error {
	return nil
}
func (m *mockDriver) ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	return nil, nil
}
func (m *mockDriver) ClusterInstall(ctx context.Context, cluster *model.Cluster) error   { return nil }
func (m *mockDriver) ClusterUninstall(ctx context.Context, cluster *model.Cluster) error { return nil }
func (m *mockDriver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	return nil, nil
}
func (m *mockDriver) VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.AppVolumeInstance, error) {
	return nil, nil
}
func (m *mockDriver) VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.AppVolumeInstance, error) {
	return nil, nil
}
func (m *mockDriver) VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	return nil
}
func (m *mockDriver) VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	return nil
}

func (m *mockDriver) VolumeClass(ctx context.Context, cls *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	return model.VolumeClass{
		CSIDriver:        "test.csi.driver",
		StorageClassName: "test-storage",
		AccessModes:      []string{"ReadWriteOnce"},
		ReclaimPolicy:    "Retain",
		VolumeMode:       "Filesystem",
		FSType:           "ext4",
		Attributes:       map[string]string{"fsType": "ext4"},
	}, nil
}

// TestComposeAppToObjectsVolumeEdgeCases tests volume parsing edge cases and error conditions
func TestComposeAppToObjectsVolumeEdgeCases(t *testing.T) {
	ctx := context.Background()

	// Mock dependencies
	svc := &model.Service{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}

	tests := []struct {
		name          string
		compose       string
		appVolumes    []model.AppVolume
		wantErr       string
		validateMount func(t *testing.T, objects []runtime.Object) // custom validation if needed
	}{
		{
			name: "bind_absolute_path_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - /absolute/path:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "default", Size: 1024}},
			wantErr:    "absolute bind volume not supported: /absolute/path",
		},
		{
			name: "bind_relative_path_success",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - ./relative/path:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "default", Size: 1024}},
			wantErr:    "", // should succeed
		},
		{
			name: "bind_no_default_volume_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - ./relative/path:/mnt
`,
			appVolumes: []model.AppVolume{}, // empty volumes
			wantErr:    "relative bind volume 'relative/path' requires at least one app volume (default) defined",
		},
		{
			name: "volume_root_path_success",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - data:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "", // should succeed with no SubPath
		},
		{
			name: "volume_sub_path_success",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - data/sub/path:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "", // should succeed with SubPath="sub/path"
		},
		{
			name: "volume_trailing_slash_empty_subpath",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - data/:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "", // should succeed with empty SubPath
		},
		{
			name: "volume_undefined_name_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - undefined:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "named volume 'undefined' referenced by 'undefined' is not defined in app volumes",
		},
		{
			name: "volume_undefined_name_with_subpath_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - undefined/sub/path:/mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "named volume 'undefined' referenced by 'undefined/sub/path' is not defined in app volumes",
		},
		{
			name: "unsupported_volume_type_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - type: tmpfs
        source: tmpfs
        target: /tmp
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "unsupported volume type: tmpfs (source=tmpfs target=/tmp)",
		},
		{
			name: "empty_source_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - type: bind
        target: /mnt
`,
			appVolumes: []model.AppVolume{{Name: "default", Size: 1024}},
			wantErr:    "field Source must not be empty",
		},
		{
			name: "empty_target_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - type: volume
        source: data
        target: ""
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "volume with empty source/target not supported",
		},
		{
			name: "colon_in_source_error",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - type: volume
        source: "data:invalid"
        target: /mnt
`,
			appVolumes: []model.AppVolume{{Name: "data", Size: 1024}},
			wantErr:    "unexpected ':' in volume source: data:invalid",
		},
		{
			name: "multiple_volume_types_success",
			compose: `
services:
  app:
    image: busybox
    volumes:
      - ./app/data:/app/data          # bind -> default volume subPath
      - db:/var/lib/db                # volume -> root mount
      - logs/app:/var/log/app         # volume -> subPath mount
      - cache/tmp:/tmp/cache          # volume -> subPath mount
`,
			appVolumes: []model.AppVolume{
				{Name: "default", Size: 1024},
				{Name: "db", Size: 2048},
				{Name: "logs", Size: 512},
				{Name: "cache", Size: 256},
			},
			wantErr: "", // should succeed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &model.App{
				Name:    "testapp",
				Compose: tt.compose,
				Volumes: tt.appVolumes,
			}

			// Create minimal volume instances if volumes are defined
			volumeInstances := map[string]VolumeInstanceInfo{}
			for _, vol := range tt.appVolumes {
				volumeInstances[vol.Name] = VolumeInstanceInfo{
					Handle: "test-handle-" + vol.Name,
					Size:   vol.Size,
				}
			}

			objects, warnings, err := ComposeAppToObjects(ctx, svc, prv, cls, app, volumeInstances, &mockDriver{})

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
				return // test passed
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(objects) == 0 {
				t.Fatal("expected objects to be generated")
			}

			t.Logf("Generated %d objects, %d warnings", len(objects), len(warnings))

			// Custom validation if provided
			if tt.validateMount != nil {
				tt.validateMount(t, objects)
			}
		})
	}
}

// TestComposeAppToObjectsVolumeSubPathCreation tests that initContainer creates correct subPath directories
func TestComposeAppToObjectsVolumeSubPathCreation(t *testing.T) {
	ctx := context.Background()

	svc := &model.Service{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}

	compose := `
services:
  app1:
    image: busybox
    volumes:
      - ./data/app1:/app/data         # default volume subPath="data/app1"
      - logs/app1:/var/log            # logs volume subPath="app1"
      - cache:/tmp/cache              # cache volume root mount (no subPath)
  app2:
    image: nginx
    volumes:
      - ./static/web:/var/www         # default volume subPath="static/web"
      - logs/app2:/var/log            # logs volume subPath="app2"
`

	app := &model.App{
		Name:    "multiapp",
		Compose: compose,
		Volumes: []model.AppVolume{
			{Name: "default", Size: 1024},
			{Name: "logs", Size: 512},
			{Name: "cache", Size: 256},
		},
	}

	volumeInstances := map[string]VolumeInstanceInfo{
		"default": {Handle: "test-handle-default", Size: 1024},
		"logs":    {Handle: "test-handle-logs", Size: 512},
		"cache":   {Handle: "test-handle-cache", Size: 256},
	}

	objects, _, err := ComposeAppToObjects(ctx, svc, prv, cls, app, volumeInstances, &mockDriver{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the Deployment and check initContainer
	var deployment *appsv1.Deployment
	for _, obj := range objects {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			deployment = dep
			break
		}
	}

	if deployment == nil {
		t.Fatal("deployment not found in generated objects")
	}

	if len(deployment.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("expected 1 initContainer, got %d", len(deployment.Spec.Template.Spec.InitContainers))
	}

	initContainer := deployment.Spec.Template.Spec.InitContainers[0]
	if initContainer.Name != "init-volume-subpaths" {
		t.Errorf("expected initContainer name 'init-volume-subpaths', got %q", initContainer.Name)
	}

	// Check that the command creates the expected subPath directories
	command := strings.Join(initContainer.Command, " ")
	expectedPaths := []string{
		"mkdir -m 1777 -p /work/default/data/app1",
		"mkdir -m 1777 -p /work/default/static/web",
		"mkdir -m 1777 -p /work/logs/app1",
		"mkdir -m 1777 -p /work/logs/app2",
	}

	for _, expectedPath := range expectedPaths {
		if !strings.Contains(command, expectedPath) {
			t.Errorf("expected command to contain %q, got %q", expectedPath, command)
		}
	}

	// Check volume mounts for initContainer
	expectedMounts := map[string]string{
		"default": "/work/default",
		"logs":    "/work/logs",
	}

	if len(initContainer.VolumeMounts) != len(expectedMounts) {
		t.Fatalf("expected %d volume mounts, got %d", len(expectedMounts), len(initContainer.VolumeMounts))
	}

	for _, vm := range initContainer.VolumeMounts {
		expectedPath, ok := expectedMounts[vm.Name]
		if !ok {
			t.Errorf("unexpected volume mount name %q", vm.Name)
		} else if vm.MountPath != expectedPath {
			t.Errorf("expected mount path %q for volume %q, got %q", expectedPath, vm.Name, vm.MountPath)
		}
	}
}
