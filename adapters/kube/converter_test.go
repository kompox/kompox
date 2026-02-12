package kube

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestNewConverter tests the basic constructor and precomputed identifiers
func TestNewConverter(t *testing.T) {
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{Name: "testapp", Compose: ""}

	c := NewConverter(svc, prv, cls, app, "app")

	if c.Svc != svc || c.Prv != prv || c.Cls != cls || c.App != app {
		t.Error("domain references not properly stored")
	}

	if c.HashSP == "" || c.HashID == "" || c.HashIN == "" {
		t.Error("hash identifiers not computed")
	}

	expectedNS := fmt.Sprintf("k4x-%s-testapp-%s", c.HashSP, c.HashID)
	if c.Namespace != expectedNS {
		t.Errorf("expected namespace %q, got %q", expectedNS, c.Namespace)
	}

	expectedResourceName := "testapp-app"
	if c.ResourceName != expectedResourceName {
		t.Errorf("expected resource name %q, got %q", expectedResourceName, c.ResourceName)
	}

	expectedLabels := map[string]string{
		LabelAppSelector:        "testapp-app",
		LabelAppK8sName:         "testapp",
		LabelAppK8sInstance:     "testapp-" + c.HashIN,
		LabelAppK8sComponent:    "app",
		LabelAppK8sManagedBy:    "kompox",
		LabelK4xAppInstanceHash: c.HashIN,
		LabelK4xAppIDHash:       c.HashID,
	}

	for k, v := range expectedLabels {
		if c.ComponentLabels[k] != v {
			t.Errorf("expected label %q=%q, got %q", k, v, c.ComponentLabels[k])
		}
	}
}

// TestNamingK4xPrefix ensures namespace and volume resource names adopt the new k4x- prefix.
func TestNamingK4xPrefix(t *testing.T) {
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}
	app := &model.App{Name: "app", Compose: "services:\n  c:\n    image: busybox\n"}

	c := NewConverter(svc, prv, cls, app, "app")
	if !strings.HasPrefix(c.Namespace, "k4x-") {
		t.Fatalf("namespace %s does not have k4x- prefix", c.Namespace)
	}
	// Simulate volume name generation via naming.NewHashes used in converter volume path.
	hashes := naming.NewHashes(svc.Name, prv.Name, cls.Name, app.Name)
	volName := hashes.VolumeResourceName("data", "handle-123")
	if !strings.HasPrefix(volName, "k4x-") {
		t.Fatalf("volume resource name %s does not have k4x- prefix", volName)
	}
}

// TestNewConverterNilInputs tests behavior with nil inputs
func TestNewConverterNilInputs(t *testing.T) {
	c := NewConverter(nil, nil, nil, nil, "app")

	if c.HashID != "" || c.HashIN != "" || c.Namespace != "" {
		t.Error("expected empty identifiers with nil inputs")
	}

	if len(c.ComponentLabels) != 0 {
		t.Error("expected empty labels with nil inputs")
	}
}

// TestConverterConvert tests the Convert phase
func TestConverterConvert(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	tests := []struct {
		name         string
		compose      string
		appVolumes   []model.AppVolume
		appIngress   model.AppIngress
		cluster      *model.Cluster
		wantErr      string
		wantWarnings int
		validate     func(t *testing.T, c *Converter)
	}{
		{
			name: "simple_app_no_volumes_no_ingress",
			compose: `
services:
  web:
    image: nginx:1.20
    environment:
      ENV_VAR: value
`,
			wantErr: "",
			validate: func(t *testing.T, c *Converter) {
				if c.Project == nil {
					t.Error("project not set")
				}
				if c.K8sNamespace == nil {
					t.Error("namespace not created")
				}
				if len(c.K8sContainers) != 1 {
					t.Errorf("expected 1 container, got %d", len(c.K8sContainers))
				}
				if c.K8sContainers[0].Name != "web" || c.K8sContainers[0].Image != "nginx:1.20" {
					t.Errorf("unexpected container: %+v", c.K8sContainers[0])
				}
				if c.K8sService != nil {
					t.Error("service should be nil when no ports defined")
				}
				if c.K8sIngressDefault != nil || c.K8sIngressCustom != nil {
					t.Error("ingress should be nil when no ingress rules defined")
				}
			},
		},
		{
			name: "app_with_ports_creates_service",
			compose: `
services:
  web:
    image: nginx:1.20
    ports:
      - "8080:80"
      - "8443:443"
`,
			wantErr: "",
			validate: func(t *testing.T, c *Converter) {
				if c.K8sService == nil {
					t.Error("service should be created when ports are defined")
					return
				}
				if len(c.K8sService.Spec.Ports) != 2 {
					t.Errorf("expected 2 service ports, got %d", len(c.K8sService.Spec.Ports))
				}
				// Check ports are sorted by host port
				if c.K8sService.Spec.Ports[0].Name != "p8080" || int(c.K8sService.Spec.Ports[0].Port) != 8080 {
					t.Errorf("unexpected first port: %+v", c.K8sService.Spec.Ports[0])
				}
				if c.K8sService.Spec.Ports[1].Name != "p8443" || int(c.K8sService.Spec.Ports[1].Port) != 8443 {
					t.Errorf("unexpected second port: %+v", c.K8sService.Spec.Ports[1])
				}
			},
		},
		{
			name: "app_with_ingress_rules",
			compose: `
services:
  web:
    image: nginx:1.20
    ports:
      - "8080:80"
`,
			appIngress: model.AppIngress{
				Rules: []model.AppIngressRule{
					{Name: "web", Port: 8080, Hosts: []string{"example.com", "www.example.com"}},
				},
			},
			wantErr: "",
			validate: func(t *testing.T, c *Converter) {
				if c.K8sService == nil {
					t.Error("service should be created when ingress rules are defined")
					return
				}
				if len(c.K8sService.Spec.Ports) != 1 {
					t.Errorf("expected 1 service port, got %d", len(c.K8sService.Spec.Ports))
				}
				if c.K8sService.Spec.Ports[0].Name != "web" {
					t.Errorf("expected service port name 'web', got %q", c.K8sService.Spec.Ports[0].Name)
				}
				if c.K8sIngressCustom == nil {
					t.Error("ingress should be created when ingress rules are defined")
					return
				}
				if len(c.K8sIngressCustom.Spec.Rules) != 2 {
					t.Errorf("expected 2 ingress rules (one per host), got %d", len(c.K8sIngressCustom.Spec.Rules))
				}
			},
		},
		{
			name: "custom_domain_host_conflicts_with_cluster_domain",
			compose: `
services:
  web:
    image: nginx:1.20
    ports:
      - "8080:80"
`,
			appIngress: model.AppIngress{
				Rules: []model.AppIngressRule{
					{Name: "web", Port: 8080, Hosts: []string{"foo.ops.kompox.dev"}},
				},
			},
			cluster: &model.Cluster{
				Name:    "testcls",
				Ingress: &model.ClusterIngress{Domain: "ops.kompox.dev"},
			},
			wantErr: "must not be under cluster ingress domain",
		},
		{
			name: "app_with_volumes_creates_initcontainer",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - ./static:/var/www
      - logs/nginx:/var/log/nginx
`,
			appVolumes: []model.AppVolume{
				{Name: "default", Size: 1024},
				{Name: "logs", Size: 512},
			},
			wantErr: "",
			validate: func(t *testing.T, c *Converter) {
				if len(c.K8sInitContainers) != 1 {
					t.Errorf("expected 1 init container for subpath creation, got %d", len(c.K8sInitContainers))
				}
				if len(c.K8sInitContainers) > 0 && c.K8sInitContainers[0].Name != "init-volume-subpaths" {
					t.Errorf("expected init container name 'init-volume-subpaths', got %q", c.K8sInitContainers[0].Name)
				}
			},
		},
		{
			name: "invalid_compose_syntax",
			compose: `
invalid yaml [
`,
			wantErr: "compose project failed",
		},
		{
			name: "empty_volume_source_error",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - type: bind
        target: /mnt
`,
			wantErr: "field Source must not be empty",
		},
		{
			name: "undefined_volume_error",
			compose: `
services:
  web:
    image: nginx:1.20
    volumes:
      - undefined:/mnt
`,
			wantErr: "named volume 'undefined'",
		},
		{
			name: "port_conflict_error",
			compose: `
services:
  web1:
    image: nginx:1.20
    ports:
      - "8080:80"
  web2:
    image: nginx:1.21
    ports:
      - "8080:8080"
`,
			wantErr: "hostPort 8080 mapped to multiple container ports",
		},
		{
			name: "ingress_rule_invalid_name",
			compose: `
services:
  web:
    image: nginx:1.20
    ports:
      - "8080:80"
`,
			appIngress: model.AppIngress{
				Rules: []model.AppIngressRule{
					{Name: "Invalid-Name", Port: 8080, Hosts: []string{"example.com"}},
				},
			},
			wantErr: "invalid ingress name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &model.Workspace{Name: "testsvc"}
			prv := &model.Provider{Name: "testprv", Driver: "test"}
			cls := tt.cluster
			if cls == nil {
				cls = &model.Cluster{Name: "testcls"}
			}
			app := &model.App{
				Name:    "testapp",
				Compose: tt.compose,
				RefBase: "file://" + cwd + "/",
				Volumes: tt.appVolumes,
				Ingress: tt.appIngress,
			}

			c := NewConverter(svc, prv, cls, app, "app")
			warnings, err := c.Convert(ctx)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantWarnings > 0 && len(warnings) != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, len(warnings))
			}

			if tt.validate != nil {
				tt.validate(t, c)
			}
		})
	}
}

// TestConverterBindVolumes tests the BindVolumes phase
func TestConverterBindVolumes(t *testing.T) {
	ctx := context.Background()

	compose := `
services:
  web:
    image: nginx:1.20
    volumes:
      - data:/var/lib/data
      - cache:/tmp/cache
`

	cwd, _ := os.Getwd()
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{
		Name:    "testapp",
		Compose: compose,
		RefBase: "file://" + cwd + "/",
		Volumes: []model.AppVolume{
			{Name: "data", Size: 1024},
			{Name: "cache", Size: 512},
		},
	}

	c := NewConverter(svc, prv, cls, app, "app")
	_, err := c.Convert(ctx)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	tests := []struct {
		name        string
		bindings    []*ConverterVolumeBinding
		wantErr     string
		wantPVCount int
	}{
		{
			name: "successful_binding",
			bindings: []*ConverterVolumeBinding{
				{
					Name: "data",
					VolumeDisk: &model.VolumeDisk{
						Handle: "test-handle-data",
						Size:   1024,
					},
					VolumeClass: &model.VolumeClass{
						CSIDriver:        "test.csi.driver",
						StorageClassName: "test-storage",
						AccessModes:      []string{"ReadWriteOnce"},
						ReclaimPolicy:    "Retain",
						VolumeMode:       "Filesystem",
						FSType:           "ext4",
					},
				},
				{
					Name: "cache",
					VolumeDisk: &model.VolumeDisk{
						Handle: "test-handle-cache",
						Size:   512,
					},
					VolumeClass: &model.VolumeClass{
						CSIDriver:        "test.csi.driver",
						StorageClassName: "test-storage",
						AccessModes:      []string{"ReadWriteOnce"},
						ReclaimPolicy:    "Retain",
						VolumeMode:       "Filesystem",
						FSType:           "ext4",
					},
				},
			},
			wantPVCount: 4, // 2 PVs + 2 PVCs
		},
		// wrong_binding_count no longer errors in BindVolumes; it will be validated in Build
		{
			name: "empty_handle_error",
			bindings: []*ConverterVolumeBinding{
				{Name: "data", VolumeDisk: &model.VolumeDisk{Handle: ""}},
				{Name: "cache", VolumeDisk: &model.VolumeDisk{Handle: "test-handle-cache"}},
			},
			wantErr: "volume data has no handle in binding input",
		},
		{
			name: "no_csi_driver_error",
			bindings: []*ConverterVolumeBinding{
				{
					Name:        "data",
					VolumeDisk:  &model.VolumeDisk{Handle: "test-handle-data"},
					VolumeClass: nil, // missing CSIDriver via nil VolumeClass
				},
				{Name: "cache", VolumeDisk: &model.VolumeDisk{Handle: "test-handle-cache"}},
			},
			wantErr: "no VolumeClass for volume data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh converter for each test
			testC := NewConverter(svc, prv, cls, app, "app")
			_, err := testC.Convert(ctx)
			if err != nil {
				t.Fatalf("convert failed: %v", err)
			}

			err = testC.BindVolumes(ctx, tt.bindings)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			totalPVObjects := len(testC.K8sPVs) + len(testC.K8sPVCs)
			if totalPVObjects != tt.wantPVCount {
				t.Errorf("expected %d PV/PVC objects, got %d", tt.wantPVCount, totalPVObjects)
			}

			if len(testC.VolumeBindings) != len(tt.bindings) {
				t.Errorf("expected %d volume bindings, got %d", len(tt.bindings), len(testC.VolumeBindings))
			}

			// Check that resource names are populated
			for i, vb := range testC.VolumeBindings {
				if strings.TrimSpace(vb.ResourceName) == "" {
					t.Errorf("binding %d has empty resource name", i)
				}
			}
		})
	}

	// Additional: Bind fewer than app volumes should succeed here and fail at Build
	{ // scope for variables
		testC := NewConverter(svc, prv, cls, app, "app")
		_, err := testC.Convert(ctx)
		if err != nil {
			t.Fatalf("convert failed: %v", err)
		}
		few := []*ConverterVolumeBinding{
			{
				Name:        "data",
				VolumeDisk:  &model.VolumeDisk{Handle: "only-data"},
				VolumeClass: &model.VolumeClass{CSIDriver: "test.csi"},
			},
		}
		if err := testC.BindVolumes(ctx, few); err != nil {
			t.Fatalf("BindVolumes should accept fewer bindings, got error: %v", err)
		}
		if _, err := testC.Build(); err == nil || !strings.Contains(err.Error(), "volume bindings count") {
			t.Fatalf("Build should fail due to insufficient bindings, got: %v", err)
		}
	}

	// Additional: Bind more than app volumes is allowed if names are valid; here last has empty name so it should error
	{ // scope
		testC := NewConverter(svc, prv, cls, app, "app")
		_, err := testC.Convert(ctx)
		if err != nil {
			t.Fatalf("convert failed: %v", err)
		}
		more := []*ConverterVolumeBinding{
			{Name: "data", VolumeDisk: &model.VolumeDisk{Handle: "h1"}, VolumeClass: &model.VolumeClass{CSIDriver: "test.csi"}},
			{Name: "cache", VolumeDisk: &model.VolumeDisk{Handle: "h2"}, VolumeClass: &model.VolumeClass{CSIDriver: "test.csi"}},
			{ /* empty name not allowed */ VolumeDisk: &model.VolumeDisk{Handle: "h3"}, VolumeClass: &model.VolumeClass{CSIDriver: "test.csi"}},
		}
		if err := testC.BindVolumes(ctx, more); err == nil || !strings.Contains(err.Error(), "empty name") {
			t.Fatalf("expected empty name error for extra binding, got: %v", err)
		}
	}
}

// TestConverterBuild tests the final Build phase
func TestConverterBuild(t *testing.T) {
	ctx := context.Background()

	compose := `
services:
  web:
    image: nginx:1.20
    ports:
      - "8080:80"
    volumes:
      - data:/var/lib/data
    environment:
      ENV_VAR: test_value
`

	cwd, _ := os.Getwd()
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{
		Name:    "testapp",
		Compose: compose,
		RefBase: "file://" + cwd + "/",
		Volumes: []model.AppVolume{{Name: "data", Size: 1024}},
		Ingress: model.AppIngress{
			Rules: []model.AppIngressRule{
				{Name: "web", Port: 8080, Hosts: []string{"example.com"}},
			},
		},
	}

	c := NewConverter(svc, prv, cls, app, "app")

	// Convert phase
	_, err := c.Convert(ctx)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	// Template bindings used per test (fresh copy each time)
	mkBindings := func() []*ConverterVolumeBinding {
		return []*ConverterVolumeBinding{
			{
				Name:       "data",
				VolumeDisk: &model.VolumeDisk{Handle: "test-handle-data"},
				VolumeClass: &model.VolumeClass{
					CSIDriver:        "test.csi.driver",
					StorageClassName: "test-storage",
					AccessModes:      []string{"ReadWriteOnce"},
					ReclaimPolicy:    "Retain",
					VolumeMode:       "Filesystem",
					FSType:           "ext4",
				},
			},
		}
	}

	tests := []struct {
		name     string
		setup    func(c *Converter)
		wantErr  string
		validate func(t *testing.T, objects []runtime.Object, warnings []string)
	}{
		{
			name:  "successful_build",
			setup: func(c *Converter) {}, // no modification needed
			validate: func(t *testing.T, objects []runtime.Object, warnings []string) {
				// Now includes 1 headless service in addition to prior objects => 11
				if len(objects) != 11 { // Namespace + SA + Role + RoleBinding + NP + PV + PVC + Deployment + Service + Headless + Ingress
					t.Errorf("expected 11 objects, got %d", len(objects))
				}

				// Find and validate Deployment
				var deployment *appsv1.Deployment
				var namespace *corev1.Namespace
				var pv *corev1.PersistentVolume
				var pvc *corev1.PersistentVolumeClaim
				var services []*corev1.Service
				for _, obj := range objects {
					switch o := obj.(type) {
					case *appsv1.Deployment:
						deployment = o
					case *corev1.Namespace:
						namespace = o
					case *corev1.PersistentVolume:
						pv = o
					case *corev1.PersistentVolumeClaim:
						pvc = o
					case *corev1.Service:
						services = append(services, o)
					}
				}
				if deployment == nil {
					t.Error("deployment not found")
					return
				}
				if len(services) != 2 { // main + headless
					t.Fatalf("expected 2 services (main + headless), got %d", len(services))
				}
				// Identify headless vs main
				var headlessFound int
				var mainFound int
				for _, s := range services {
					if s.Labels[LabelK4xComposeServiceHeadless] == "true" {
						headlessFound++
						if s.Spec.ClusterIP != corev1.ClusterIPNone {
							t.Errorf("headless service %s should have ClusterIP None", s.Name)
						}
						if len(s.Spec.Ports) != 0 {
							t.Errorf("headless service %s should have 0 ports", s.Name)
						}
						// selector must NOT include marker label
						if _, ok := s.Spec.Selector[LabelK4xComposeServiceHeadless]; ok {
							t.Errorf("headless service %s selector must not contain marker label", s.Name)
						}
					} else {
						mainFound++
					}
				}
				if headlessFound != 1 || mainFound != 1 {
					t.Errorf("expected 1 headless and 1 main service, got headless=%d main=%d", headlessFound, mainFound)
				}

				// Ensure Namespace does NOT have component-specific labels
				if namespace == nil {
					t.Error("namespace not found")
				} else {
					if _, ok := namespace.Labels[LabelAppSelector]; ok {
						t.Error("namespace must not have component 'app' selector label")
					}
					if _, ok := namespace.Labels[LabelAppK8sComponent]; ok {
						t.Error("namespace must not have component label")
					}
				}

				// Ensure PV/PVC do NOT have component-specific labels
				if pv != nil {
					if _, ok := pv.Labels[LabelAppSelector]; ok {
						t.Error("persistentVolume must not have component selector label")
					}
					if _, ok := pv.Labels[LabelAppK8sComponent]; ok {
						t.Error("persistentVolume must not have component label")
					}
				}
				if pvc != nil {
					if _, ok := pvc.Labels[LabelAppSelector]; ok {
						t.Error("persistentVolumeClaim must not have component selector label")
					}
					if _, ok := pvc.Labels[LabelAppK8sComponent]; ok {
						t.Error("persistentVolumeClaim must not have component label")
					}
				}

				if len(deployment.Spec.Template.Spec.Containers) != 1 {
					t.Errorf("expected 1 container in deployment, got %d", len(deployment.Spec.Template.Spec.Containers))
				}

				if len(deployment.Spec.Template.Spec.Volumes) != 1 {
					t.Errorf("expected 1 volume in deployment, got %d", len(deployment.Spec.Template.Spec.Volumes))
				}

				container := deployment.Spec.Template.Spec.Containers[0]
				if container.Name != "web" || container.Image != "nginx:1.20" {
					t.Errorf("unexpected container: name=%q image=%q", container.Name, container.Image)
				}

				// Check environment variables
				if len(container.Env) != 1 {
					t.Errorf("expected 1 env var, got %d", len(container.Env))
				} else if container.Env[0].Name != "ENV_VAR" || container.Env[0].Value != "test_value" {
					t.Errorf("unexpected env var: %+v", container.Env[0])
				}
			},
		},
		{
			name: "build_before_convert_error",
			setup: func(c *Converter) {
				c.Project = nil // simulate not converted
			},
			wantErr: "convert must be called before build",
		},
		{
			name: "build_before_bind_error",
			setup: func(c *Converter) {
				c.VolumeBindings = nil // simulate not bound
			},
			wantErr: "volume bindings count",
		},
		{
			name: "build_name_mismatch_error",
			setup: func(c *Converter) {
				// Corrupt the binding name to not match the app volume order/name
				if len(c.VolumeBindings) == 1 {
					c.VolumeBindings[0].Name = "wrong-name"
				}
			},
			wantErr: "volume binding at index 0 is for wrong-name; expected data",
		},
		{
			name: "build_with_insufficient_bindings_errors",
			setup: func(c *Converter) {
				// Provide only 0 bindings while app defines 1 volume
				c.VolumeBindings = []*ConverterVolumeBinding{}
			},
			wantErr: "volume bindings count 0 does not match app volumes 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh converter for each test
			testC := NewConverter(svc, prv, cls, app, "app")
			_, err := testC.Convert(ctx)
			if err != nil {
				t.Fatalf("convert failed: %v", err)
			}
			bindings := mkBindings()
			err = testC.BindVolumes(ctx, bindings)
			if err != nil {
				t.Fatalf("bind volumes failed: %v", err)
			}

			if tt.setup != nil {
				tt.setup(testC)
			}

			warnings, err := testC.Build()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			objects := testC.AllObjects()
			if tt.validate != nil {
				tt.validate(t, objects, warnings)
			}
		})
	}
}

// TestConverterFullWorkflow tests the complete workflow: NewConverter -> Convert -> BindVolumes -> Build
func TestConverterFullWorkflow(t *testing.T) {
	ctx := context.Background()

	compose := `
services:
  frontend:
    image: nginx:1.20
    ports:
      - "8080:80"
    volumes:
      - ./static:/usr/share/nginx/html
      - logs:/var/log/nginx
    environment:
      NGINX_PORT: "80"
  backend:
    image: node:18
    ports:
      - "3000:3000"
    volumes:
      - ./app:/app
      - logs/backend:/var/log/app
    environment:
      NODE_ENV: production
      PORT: "3000"
`

	cwd, _ := os.Getwd()
	svc := &model.Workspace{Name: "myservice"}
	prv := &model.Provider{Name: "myprovider", Driver: "test"}
	cls := &model.Cluster{Name: "mycluster", Ingress: &model.ClusterIngress{Domain: "ops.kompox.dev"}}
	app := &model.App{
		Name:    "fullapp",
		Compose: compose,
		RefBase: "file://" + cwd + "/",
		Volumes: []model.AppVolume{
			{Name: "default", Size: 2048},
			{Name: "logs", Size: 1024},
		},
		Ingress: model.AppIngress{
			Rules: []model.AppIngressRule{
				{Name: "frontend", Port: 8080, Hosts: []string{"myapp.example.com"}},
				{Name: "api", Port: 3000, Hosts: []string{"api.example.com"}},
			},
			CertResolver: "letsencrypt",
		},
	}

	// Step 1: Create converter
	c := NewConverter(svc, prv, cls, app, "app")
	if c == nil {
		t.Fatal("failed to create converter")
	}

	// Step 2: Convert compose
	warnings, err := c.Convert(ctx)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	t.Logf("Convert warnings: %v", warnings)

	// Step 3: Bind volumes
	bindings := []*ConverterVolumeBinding{
		{
			Name:       "default",
			VolumeDisk: &model.VolumeDisk{Handle: "azure-disk-handle-default"},
			VolumeClass: &model.VolumeClass{
				CSIDriver:        "disk.csi.azure.com",
				StorageClassName: "managed-csi",
				AccessModes:      []string{"ReadWriteOnce"},
				ReclaimPolicy:    "Retain",
				VolumeMode:       "Filesystem",
				FSType:           "ext4",
			},
		},
		{
			Name:       "logs",
			VolumeDisk: &model.VolumeDisk{Handle: "azure-disk-handle-logs"},
			VolumeClass: &model.VolumeClass{
				CSIDriver:        "disk.csi.azure.com",
				StorageClassName: "managed-csi",
				AccessModes:      []string{"ReadWriteOnce"},
				ReclaimPolicy:    "Retain",
				VolumeMode:       "Filesystem",
				FSType:           "ext4",
			},
		},
	}

	err = c.BindVolumes(ctx, bindings)
	if err != nil {
		t.Fatalf("bind volumes failed: %v", err)
	}

	// Step 4: Build final objects
	buildWarnings, err := c.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Logf("Build warnings: %v", buildWarnings)
	objects := c.AllObjects()
	// Validate the final result
	if len(objects) != 15 { // +2 headless services (frontend, backend)
		t.Errorf("expected 15 objects, got %d", len(objects))
	}

	// Count object types
	counts := map[string]int{}
	for _, obj := range objects {
		switch obj.(type) {
		case *corev1.Namespace:
			counts["Namespace"]++
		case *corev1.ServiceAccount:
			counts["ServiceAccount"]++
		case *rbacv1.Role:
			counts["Role"]++
		case *rbacv1.RoleBinding:
			counts["RoleBinding"]++
		case *netv1.NetworkPolicy:
			counts["NetworkPolicy"]++
		case *corev1.PersistentVolume:
			counts["PersistentVolume"]++
		case *corev1.PersistentVolumeClaim:
			counts["PersistentVolumeClaim"]++
		case *appsv1.Deployment:
			counts["Deployment"]++
		case *corev1.Service:
			counts["Service"]++
		case *netv1.Ingress:
			counts["Ingress"]++
		default:
			t.Errorf("unexpected object type: %T", obj)
		}
	}

	expected := map[string]int{
		"Namespace":             1,
		"ServiceAccount":        1,
		"Role":                  1,
		"RoleBinding":           1,
		"NetworkPolicy":         1,
		"PersistentVolume":      2,
		"PersistentVolumeClaim": 2,
		"Deployment":            1,
		"Service":               3, // main + 2 headless
		"Ingress":               2,
	}

	for objType, expectedCount := range expected {
		if counts[objType] != expectedCount {
			t.Errorf("expected %d %s, got %d", expectedCount, objType, counts[objType])
		}
	}

	// Find and validate key objects
	var deployment *appsv1.Deployment
	var services []*corev1.Service
	var ingDefault *netv1.Ingress
	var ingCustom *netv1.Ingress
	var namespace *corev1.Namespace
	var pv1 *corev1.PersistentVolume
	var pvc1 *corev1.PersistentVolumeClaim

	for _, obj := range objects {
		switch v := obj.(type) {
		case *appsv1.Deployment:
			deployment = v
		case *corev1.Service:
			services = append(services, v)
		case *netv1.Ingress:
			switch v.Name {
			case "fullapp-app-default":
				ingDefault = v
			case "fullapp-app-custom":
				ingCustom = v
			}
		case *corev1.Namespace:
			namespace = v
		case *corev1.PersistentVolume:
			if pv1 == nil {
				pv1 = v
			}
		case *corev1.PersistentVolumeClaim:
			if pvc1 == nil {
				pvc1 = v
			}
		}
	}

	// Validate deployment
	if deployment == nil {
		t.Fatal("deployment not found")
	}
	if deployment.Name != "fullapp-app" {
		t.Errorf("expected deployment name 'fullapp-app', got %q", deployment.Name)
	}
	if len(deployment.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(deployment.Spec.Template.Spec.Containers))
	}
	if len(deployment.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("expected 1 init container for subpaths, got %d", len(deployment.Spec.Template.Spec.InitContainers))
	}

	// Validate service
	if len(services) != 3 {
		t.Fatalf("expected 3 services (1 main + 2 headless), got %d", len(services))
	}
	// identify main service
	var main *corev1.Service
	var headlessCount int
	for _, s := range services {
		if s.Labels[LabelK4xComposeServiceHeadless] == "true" {
			headlessCount++
			if s.Spec.ClusterIP != corev1.ClusterIPNone {
				t.Errorf("headless service %s should have ClusterIP None", s.Name)
			}
			if len(s.Spec.Ports) != 0 {
				t.Errorf("headless service %s should have 0 ports", s.Name)
			}
			if _, ok := s.Spec.Selector[LabelK4xComposeServiceHeadless]; ok {
				t.Errorf("headless service %s selector must not contain marker label", s.Name)
			}
		} else {
			main = s
		}
	}
	if headlessCount != 2 {
		t.Errorf("expected 2 headless services, got %d", headlessCount)
	}
	if main == nil {
		t.Fatalf("main service not found")
	}
	if main.Name != "fullapp-app" {
		t.Errorf("expected main service name 'fullapp-app', got %q", main.Name)
	}
	if len(main.Spec.Ports) != 2 {
		t.Errorf("expected 2 service ports, got %d", len(main.Spec.Ports))
	}

	// Validate ingress
	if ingDefault == nil || ingCustom == nil {
		t.Fatalf("both default and custom ingresses should exist (default:%v custom:%v)", ingDefault != nil, ingCustom != nil)
	}
	if len(ingDefault.Spec.Rules) != 2 { // 1 per rule using default domain
		t.Errorf("expected 2 default ingress rules, got %d", len(ingDefault.Spec.Rules))
	}
	if len(ingCustom.Spec.Rules) != 2 { // 1 per explicit host
		t.Errorf("expected 2 custom ingress rules, got %d", len(ingCustom.Spec.Rules))
	}

	// Check Traefik annotations on custom (certresolver set), default (no certresolver)
	if _, ok := ingDefault.Annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"]; ok {
		t.Errorf("default ingress should not have certresolver")
	}
	certResolver, ok := ingCustom.Annotations["traefik.ingress.kubernetes.io/router.tls.certresolver"]
	if !ok || certResolver != "letsencrypt" {
		t.Errorf("expected cert resolver 'letsencrypt' on custom ingress, got %q", certResolver)
	}

	// Namespace/PV/PVC must not have component-specific labels
	if namespace == nil {
		t.Fatal("namespace not found")
	}
	if _, ok := namespace.Labels[LabelAppSelector]; ok {
		t.Error("namespace must not have component selector label")
	}
	if _, ok := namespace.Labels[LabelAppK8sComponent]; ok {
		t.Error("namespace must not have component label")
	}
	if pv1 != nil {
		if _, ok := pv1.Labels[LabelAppSelector]; ok {
			t.Error("persistentVolume must not have component selector label")
		}
		if _, ok := pv1.Labels[LabelAppK8sComponent]; ok {
			t.Error("persistentVolume must not have component label")
		}
	}
	if pvc1 != nil {
		if _, ok := pvc1.Labels[LabelAppSelector]; ok {
			t.Error("persistentVolumeClaim must not have component selector label")
		}
		if _, ok := pvc1.Labels[LabelAppK8sComponent]; ok {
			t.Error("persistentVolumeClaim must not have component label")
		}
	}

	t.Logf("Full workflow test completed successfully with %d objects", len(objects))
}

// TestDeploymentNodeSelector tests the nodeSelector functionality from app.deployment spec
func TestDeploymentNodeSelector(t *testing.T) {
	cwd, _ := os.Getwd()
	svc := &model.Workspace{Name: "ops"}
	prv := &model.Provider{Name: "aks1", Driver: "aks"}
	cls := &model.Cluster{Name: "cluster1"}

	// Test case 1: Default node pool (no deployment config)
	app1 := &model.App{
		Name:    "app1",
		Compose: `services: {app: {image: "test"}}`,
		RefBase: "file://" + cwd + "/",
	}
	c1 := NewConverter(svc, prv, cls, app1, "app")
	warnings, err := c1.Convert(context.Background())
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}

	_, err = c1.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	objects := c1.AllObjects()

	// Find deployment
	var deployment *appsv1.Deployment
	for _, obj := range objects {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			deployment = dep
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	// Check default pool
	nodeSelector := deployment.Spec.Template.Spec.NodeSelector
	if nodeSelector[LabelK4xNodePool] != "user" {
		t.Errorf("expected default node pool 'user', got %q", nodeSelector[LabelK4xNodePool])
	}
	if _, hasZone := nodeSelector[LabelK4xNodeZone]; hasZone {
		t.Errorf("expected no zone selector by default, but found: %q", nodeSelector[LabelK4xNodeZone])
	}

	// Test case 2: Custom pool and zone
	app2 := &model.App{
		Name:    "app2",
		Compose: `services: {app: {image: "test"}}`,
		RefBase: "file://" + cwd + "/",
		Deployment: model.AppDeployment{
			Pool: "system",
			Zone: "japaneast-1",
		},
	}
	c2 := NewConverter(svc, prv, cls, app2, "app")
	_, err = c2.Convert(context.Background())
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	_, err = c2.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	objects = c2.AllObjects()

	// Find deployment
	deployment = nil
	for _, obj := range objects {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			deployment = dep
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	// Check custom pool and zone
	nodeSelector = deployment.Spec.Template.Spec.NodeSelector
	if nodeSelector[LabelK4xNodePool] != "system" {
		t.Errorf("expected node pool 'system', got %q", nodeSelector[LabelK4xNodePool])
	}
	if nodeSelector[LabelK4xNodeZone] != "japaneast-1" {
		t.Errorf("expected node zone 'japaneast-1', got %q", nodeSelector[LabelK4xNodeZone])
	}

	// Test case 3: Only zone specified (pool should default to "user")
	app3 := &model.App{
		Name:    "app3",
		Compose: `services: {app: {image: "test"}}`,
		RefBase: "file://" + cwd + "/",
		Deployment: model.AppDeployment{
			Zone: "westus2-2",
		},
	}
	c3 := NewConverter(svc, prv, cls, app3, "app")
	_, err = c3.Convert(context.Background())
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	_, err = c3.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	objects = c3.AllObjects()

	// Find deployment
	deployment = nil
	for _, obj := range objects {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			deployment = dep
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	// Check default pool with custom zone
	nodeSelector = deployment.Spec.Template.Spec.NodeSelector
	if nodeSelector[LabelK4xNodePool] != "user" {
		t.Errorf("expected default node pool 'user', got %q", nodeSelector[LabelK4xNodePool])
	}
	if nodeSelector[LabelK4xNodeZone] != "westus2-2" {
		t.Errorf("expected node zone 'westus2-2', got %q", nodeSelector[LabelK4xNodeZone])
	}

	t.Logf("All deployment nodeSelector tests completed successfully")
}

// TestHeadlessServicesGeneration validates generation details and pruning metadata.
func TestHeadlessServicesGeneration(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	compose := `
services:
  api:
    image: nginx:1.20
  worker:
    image: busybox:1.36
`
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}
	app := &model.App{Name: "demo", Compose: compose, RefBase: "file://" + cwd + "/"}
	c := NewConverter(svc, prv, cls, app, "app")
	if _, err := c.Convert(ctx); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(c.K8sHeadlessServices) != 2 {
		t.Fatalf("expected 2 headless services, got %d", len(c.K8sHeadlessServices))
	}
	names := []string{c.K8sHeadlessServices[0].Name, c.K8sHeadlessServices[1].Name}
	sort.Strings(names)
	if names[0] != "api" || names[1] != "worker" {
		t.Errorf("unexpected headless service names %v", names)
	}
	for _, hs := range c.K8sHeadlessServices {
		if hs.Spec.ClusterIP != corev1.ClusterIPNone {
			t.Errorf("headless %s missing ClusterIP None", hs.Name)
		}
		if len(hs.Spec.Ports) != 0 {
			t.Errorf("headless %s should have 0 ports", hs.Name)
		}
		if hs.Labels[LabelK4xComposeServiceHeadless] != "true" {
			t.Errorf("headless %s missing marker label", hs.Name)
		}
		if _, ok := hs.Spec.Selector[LabelK4xComposeServiceHeadless]; ok {
			t.Errorf("headless %s selector must not have marker", hs.Name)
		}
		if hs.Spec.Selector[LabelAppSelector] == "" {
			t.Errorf("headless %s selector missing component app label", hs.Name)
		}
	}
	// Pruning selector metadata
	if c.HeadlessServiceSelector[LabelAppSelector] == "" {
		t.Error("selector missing app label")
	}
	if c.HeadlessServiceSelector[LabelK4xComposeServiceHeadless] != "true" {
		t.Error("selector missing marker label")
	}
	if !strings.Contains(c.HeadlessServiceSelectorString, LabelK4xComposeServiceHeadless+"=true") {
		t.Errorf("selector string missing marker: %s", c.HeadlessServiceSelectorString)
	}
}

// TestZeroHeadlessServices ensures metadata still present when no compose services defined.
func TestZeroHeadlessServices(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()
	compose := `services: {}`
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}
	app := &model.App{Name: "empty", Compose: compose, RefBase: "file://" + cwd + "/"}
	c := NewConverter(svc, prv, cls, app, "app")
	if _, err := c.Convert(ctx); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(c.K8sHeadlessServices) != 0 {
		t.Fatalf("expected 0 headless services, got %d", len(c.K8sHeadlessServices))
	}
	if c.HeadlessServiceSelector[LabelAppSelector] == "" || c.HeadlessServiceSelector[LabelK4xComposeServiceHeadless] != "true" {
		t.Fatalf("headless selector metadata incomplete: %#v", c.HeadlessServiceSelector)
	}
}

// TestConverterEntrypointCommand tests entrypoint and command conversion.
func TestConverterEntrypointCommand(t *testing.T) {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	tests := []struct {
		name          string
		compose       string
		wantCommand   []string
		wantArgs      []string
		wantNoCommand bool
		wantNoArgs    bool
	}{
		{
			name: "both_entrypoint_and_command",
			compose: `
services:
  app:
    image: app
    entrypoint: ["/app/entrypoint.sh"]
    command: ["--config", "/etc/app.conf"]
`,
			wantCommand: []string{"/app/entrypoint.sh"},
			wantArgs:    []string{"--config", "/etc/app.conf"},
		},
		{
			name: "entrypoint_only",
			compose: `
services:
  app:
    image: app
    entrypoint: ["/bin/bash", "-c"]
`,
			wantCommand: []string{"/bin/bash", "-c"},
			wantNoArgs:  true,
		},
		{
			name: "command_only",
			compose: `
services:
  app:
    image: app
    command: ["npm", "start"]
`,
			wantNoCommand: true,
			wantArgs:      []string{"npm", "start"},
		},
		{
			name: "neither_specified",
			compose: `
services:
  app:
    image: app
`,
			wantNoCommand: true,
			wantNoArgs:    true,
		},
		{
			name: "entrypoint_single_element",
			compose: `
services:
  app:
    image: app
    entrypoint: ["/app/start"]
`,
			wantCommand: []string{"/app/start"},
			wantNoArgs:  true,
		},
		{
			name: "command_with_flags",
			compose: `
services:
  app:
    image: app
    entrypoint: ["/usr/bin/python"]
    command: ["-m", "http.server", "8000"]
`,
			wantCommand: []string{"/usr/bin/python"},
			wantArgs:    []string{"-m", "http.server", "8000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &model.Workspace{Name: "svc"}
			prv := &model.Provider{Name: "prv", Driver: "test"}
			cls := &model.Cluster{Name: "cls"}
			app := &model.App{Name: "testapp", Compose: tt.compose, RefBase: "file://" + cwd + "/"}

			c := NewConverter(svc, prv, cls, app, "app")
			_, err := c.Convert(ctx)
			if err != nil {
				t.Fatalf("convert failed: %v", err)
			}

			if len(c.K8sContainers) != 1 {
				t.Fatalf("expected 1 container, got %d", len(c.K8sContainers))
			}

			ctn := c.K8sContainers[0]

			// Check command
			if tt.wantNoCommand {
				if len(ctn.Command) != 0 {
					t.Errorf("expected no command, got %v", ctn.Command)
				}
			} else {
				if len(ctn.Command) != len(tt.wantCommand) {
					t.Errorf("command length mismatch: want %d, got %d", len(tt.wantCommand), len(ctn.Command))
				} else {
					for i, want := range tt.wantCommand {
						if ctn.Command[i] != want {
							t.Errorf("command[%d]: want %q, got %q", i, want, ctn.Command[i])
						}
					}
				}
			}

			// Check args
			if tt.wantNoArgs {
				if len(ctn.Args) != 0 {
					t.Errorf("expected no args, got %v", ctn.Args)
				}
			} else {
				if len(ctn.Args) != len(tt.wantArgs) {
					t.Errorf("args length mismatch: want %d, got %d", len(tt.wantArgs), len(ctn.Args))
				} else {
					for i, want := range tt.wantArgs {
						if ctn.Args[i] != want {
							t.Errorf("args[%d]: want %q, got %q", i, want, ctn.Args[i])
						}
					}
				}
			}
		})
	}
}

func TestNetworkPolicyWithIngressRules(t *testing.T) {
	ctx := context.Background()

	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{
		Name: "testcls",
		Ingress: &model.ClusterIngress{
			Namespace:  "traefik",
			Controller: "traefik",
			Domain:     "example.com",
		},
	}

	// App with custom NetworkPolicy rules
	app := &model.App{
		Name: "netpolapp",
		Compose: `
services:
  web:
    image: nginx:latest
`,
		NetworkPolicy: model.AppNetworkPolicy{
			IngressRules: []model.AppNetworkPolicyIngressRule{
				{
					From: []model.AppNetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "monitoring",
								},
							},
						},
					},
					Ports: []model.AppNetworkPolicyPort{
						{Protocol: "TCP", Port: 9090},
					},
				},
				{
					From: []model.AppNetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"environment": "production",
								},
							},
						},
					},
					Ports: []model.AppNetworkPolicyPort{
						{Protocol: "TCP", Port: 8080},
						{Protocol: "UDP", Port: 8081},
					},
				},
			},
		},
	}

	c := NewConverter(svc, prv, cls, app, "app")
	_, err := c.Convert(ctx)
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	_, err = c.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	objects := c.AllObjects()

	// Find NetworkPolicy
	var netpol *netv1.NetworkPolicy
	for _, obj := range objects {
		if np, ok := obj.(*netv1.NetworkPolicy); ok {
			netpol = np
			break
		}
	}

	if netpol == nil {
		t.Fatal("NetworkPolicy not found")
	}

	// Validate basic properties
	if netpol.Name != "netpolapp" {
		t.Errorf("expected NetworkPolicy name 'netpolapp', got %q", netpol.Name)
	}

	// Validate ingress rules count: 1 default + 2 user-defined = 3 total
	if len(netpol.Spec.Ingress) != 3 {
		t.Fatalf("expected 3 ingress rules (1 default + 2 custom), got %d", len(netpol.Spec.Ingress))
	}

	// First rule is the default (same-namespace + kube-system + traefik)
	defaultRule := netpol.Spec.Ingress[0]
	if len(defaultRule.From) != 2 {
		t.Errorf("expected 2 peers in default rule (pod selector + namespace selector), got %d", len(defaultRule.From))
	}
	if len(defaultRule.Ports) != 0 {
		t.Errorf("expected 0 ports in default rule (allow all), got %d", len(defaultRule.Ports))
	}

	// Second rule is the first user-defined rule (monitoring namespace, TCP 9090)
	rule1 := netpol.Spec.Ingress[1]
	if len(rule1.From) != 1 {
		t.Fatalf("expected 1 peer in first custom rule, got %d", len(rule1.From))
	}
	if rule1.From[0].NamespaceSelector == nil {
		t.Fatal("expected namespaceSelector in first custom rule")
	}
	if rule1.From[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] != "monitoring" {
		t.Errorf("expected monitoring namespace, got %v", rule1.From[0].NamespaceSelector.MatchLabels)
	}
	if len(rule1.Ports) != 1 {
		t.Fatalf("expected 1 port in first custom rule, got %d", len(rule1.Ports))
	}
	if *rule1.Ports[0].Protocol != corev1.ProtocolTCP {
		t.Errorf("expected TCP protocol, got %s", *rule1.Ports[0].Protocol)
	}
	if rule1.Ports[0].Port.IntVal != 9090 {
		t.Errorf("expected port 9090, got %d", rule1.Ports[0].Port.IntVal)
	}

	// Third rule is the second user-defined rule (production environment, TCP 8080 + UDP 8081)
	rule2 := netpol.Spec.Ingress[2]
	if len(rule2.From) != 1 {
		t.Fatalf("expected 1 peer in second custom rule, got %d", len(rule2.From))
	}
	if rule2.From[0].NamespaceSelector == nil {
		t.Fatal("expected namespaceSelector in second custom rule")
	}
	if rule2.From[0].NamespaceSelector.MatchLabels["environment"] != "production" {
		t.Errorf("expected production environment, got %v", rule2.From[0].NamespaceSelector.MatchLabels)
	}
	if len(rule2.Ports) != 2 {
		t.Fatalf("expected 2 ports in second custom rule, got %d", len(rule2.Ports))
	}
	if *rule2.Ports[0].Protocol != corev1.ProtocolTCP {
		t.Errorf("expected TCP protocol for first port, got %s", *rule2.Ports[0].Protocol)
	}
	if rule2.Ports[0].Port.IntVal != 8080 {
		t.Errorf("expected port 8080 for first port, got %d", rule2.Ports[0].Port.IntVal)
	}
	if *rule2.Ports[1].Protocol != corev1.ProtocolUDP {
		t.Errorf("expected UDP protocol for second port, got %s", *rule2.Ports[1].Protocol)
	}
	if rule2.Ports[1].Port.IntVal != 8081 {
		t.Errorf("expected port 8081 for second port, got %d", rule2.Ports[1].Port.IntVal)
	}
}

