package kube

import (
	"context"
	"strings"
	"testing"

	"github.com/kompox/kompox/domain/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestNewConverter tests the basic constructor and precomputed identifiers
func TestNewConverter(t *testing.T) {
	svc := &model.Service{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{Name: "testapp", Compose: ""}

	c := NewConverter(svc, prv, cls, app, "app")

	if c.Svc != svc || c.Prv != prv || c.Cls != cls || c.App != app {
		t.Error("domain references not properly stored")
	}

	if c.HashID == "" || c.HashIN == "" {
		t.Error("hash identifiers not computed")
	}

	expectedNS := "kompox-testapp-" + c.HashID
	if c.Namespace != expectedNS {
		t.Errorf("expected namespace %q, got %q", expectedNS, c.Namespace)
	}

	expectedLabels := map[string]string{
		"app":                          "testapp-app",
		"app.kubernetes.io/name":       "testapp",
		"app.kubernetes.io/instance":   "testapp-" + c.HashIN,
		"app.kubernetes.io/managed-by": "kompox",
		"kompox.dev/app-instance-hash": c.HashIN,
		"kompox.dev/app-id-hash":       c.HashID,
	}

	for k, v := range expectedLabels {
		if c.Labels[k] != v {
			t.Errorf("expected label %q=%q, got %q", k, v, c.Labels[k])
		}
	}
}

// TestNewConverterNilInputs tests behavior with nil inputs
func TestNewConverterNilInputs(t *testing.T) {
	c := NewConverter(nil, nil, nil, nil, "app")

	if c.HashID != "" || c.HashIN != "" || c.Namespace != "" {
		t.Error("expected empty identifiers with nil inputs")
	}

	if len(c.Labels) != 0 {
		t.Error("expected empty labels with nil inputs")
	}
}

// TestConverterConvert tests the Convert phase
func TestConverterConvert(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		compose      string
		appVolumes   []model.AppVolume
		appIngress   model.AppIngress
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
			svc := &model.Service{Name: "testsvc"}
			prv := &model.Provider{Name: "testprv", Driver: "test"}
			cls := &model.Cluster{Name: "testcls"}
			app := &model.App{
				Name:    "testapp",
				Compose: tt.compose,
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

	svc := &model.Service{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{
		Name:    "testapp",
		Compose: compose,
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
		{
			name: "wrong_binding_count",
			bindings: []*ConverterVolumeBinding{
				{Name: "data", VolumeDisk: &model.VolumeDisk{Handle: "test-handle-data"}},
			},
			wantErr: "vols length 1 does not match app volumes 2",
		},
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

	svc := &model.Service{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}
	app := &model.App{
		Name:    "testapp",
		Compose: compose,
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

	// Bind volumes phase
	bindings := []*ConverterVolumeBinding{
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

	err = c.BindVolumes(ctx, bindings)
	if err != nil {
		t.Fatalf("bind volumes failed: %v", err)
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
				if len(objects) != 6 { // Namespace + PV + PVC + Deployment + Service + Ingress
					t.Errorf("expected 6 objects, got %d", len(objects))
				}

				// Check object types in expected order
				for i, obj := range objects {
					if i < 6 { // we expect 6 specific objects
						// Check basic structure rather than exact type string
						switch i {
						case 0:
							if _, ok := obj.(*corev1.Namespace); !ok {
								t.Errorf("object %d should be Namespace, got %T", i, obj)
							}
						case 1:
							if _, ok := obj.(*corev1.PersistentVolume); !ok {
								t.Errorf("object %d should be PersistentVolume, got %T", i, obj)
							}
						case 2:
							if _, ok := obj.(*corev1.PersistentVolumeClaim); !ok {
								t.Errorf("object %d should be PersistentVolumeClaim, got %T", i, obj)
							}
						case 3:
							if _, ok := obj.(*appsv1.Deployment); !ok {
								t.Errorf("object %d should be Deployment, got %T", i, obj)
							}
						case 4:
							if _, ok := obj.(*corev1.Service); !ok {
								t.Errorf("object %d should be Service, got %T", i, obj)
							}
						case 5:
							if _, ok := obj.(*netv1.Ingress); !ok {
								t.Errorf("object %d should be Ingress, got %T", i, obj)
							}
						}
					}
				}

				// Find and validate Deployment
				var deployment *appsv1.Deployment
				for _, obj := range objects {
					if dep, ok := obj.(*appsv1.Deployment); ok {
						deployment = dep
						break
					}
				}
				if deployment == nil {
					t.Error("deployment not found")
					return
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
			wantErr: "bind must be called before build",
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
			err = testC.BindVolumes(ctx, bindings)
			if err != nil {
				t.Fatalf("bind volumes failed: %v", err)
			}

			if tt.setup != nil {
				tt.setup(testC)
			}

			objects, warnings, err := testC.Build()

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

	svc := &model.Service{Name: "myservice"}
	prv := &model.Provider{Name: "myprovider", Driver: "test"}
	cls := &model.Cluster{Name: "mycluster", Ingress: &model.ClusterIngress{Domain: "ops.kompox.dev"}}
	app := &model.App{
		Name:    "fullapp",
		Compose: compose,
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
	objects, buildWarnings, err := c.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Logf("Build warnings: %v", buildWarnings)

	// Validate the final result
	if len(objects) != 9 { // Namespace + 2PV + 2PVC + Deployment + Service + 2x Ingress
		t.Errorf("expected 9 objects, got %d", len(objects))
	}

	// Count object types
	counts := map[string]int{}
	for _, obj := range objects {
		switch obj.(type) {
		case *corev1.Namespace:
			counts["Namespace"]++
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
		"PersistentVolume":      2,
		"PersistentVolumeClaim": 2,
		"Deployment":            1,
		"Service":               1,
		"Ingress":               2,
	}

	for objType, expectedCount := range expected {
		if counts[objType] != expectedCount {
			t.Errorf("expected %d %s, got %d", expectedCount, objType, counts[objType])
		}
	}

	// Find and validate key objects
	var deployment *appsv1.Deployment
	var service *corev1.Service
	var ingDefault *netv1.Ingress
	var ingCustom *netv1.Ingress

	for _, obj := range objects {
		switch v := obj.(type) {
		case *appsv1.Deployment:
			deployment = v
		case *corev1.Service:
			service = v
		case *netv1.Ingress:
			if v.Name == "fullapp-default" {
				ingDefault = v
			} else if v.Name == "fullapp-custom" {
				ingCustom = v
			}
		}
	}

	// Validate deployment
	if deployment == nil {
		t.Fatal("deployment not found")
	}
	if len(deployment.Spec.Template.Spec.Containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(deployment.Spec.Template.Spec.Containers))
	}
	if len(deployment.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("expected 1 init container for subpaths, got %d", len(deployment.Spec.Template.Spec.InitContainers))
	}

	// Validate service
	if service == nil {
		t.Fatal("service not found")
	}
	if len(service.Spec.Ports) != 2 {
		t.Errorf("expected 2 service ports, got %d", len(service.Spec.Ports))
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

	t.Logf("Full workflow test completed successfully with %d objects", len(objects))
}
