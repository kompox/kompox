package kube

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

// TestCheckTargetConflict_DuplicateConfigsSecrets tests error detection for duplicate configs/secrets targets
func TestCheckTargetConflict_DuplicateConfigsSecrets(t *testing.T) {
	mappings := []targetMapping{
		{source: "config:app-conf", target: "/etc/app/config.yaml", location: "service web"},
		{source: "secret:api-key", target: "/etc/app/config.yaml", location: "service web"},
	}

	errs, warns := checkTargetConflict("web", mappings)

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "duplicate configs/secrets") {
		t.Errorf("expected duplicate error, got: %s", errs[0])
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warns), warns)
	}
}

// TestCheckTargetConflict_VolumeVsConfigSecret tests warning for volume vs config/secret conflict
func TestCheckTargetConflict_VolumeVsConfigSecret(t *testing.T) {
	mappings := []targetMapping{
		{source: "volume:./data", target: "/app/config.json", location: "service web"},
		{source: "config:app-conf", target: "/app/config.json", location: "service web"},
	}

	errs, warns := checkTargetConflict("web", mappings)

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warns), warns)
	}
	if !strings.Contains(warns[0], "conflicts between") || !strings.Contains(warns[0], "ignoring volume") {
		t.Errorf("expected volume conflict warning, got: %s", warns[0])
	}
}

// TestCheckTargetConflict_NoConflict tests no conflict case
func TestCheckTargetConflict_NoConflict(t *testing.T) {
	mappings := []targetMapping{
		{source: "config:app-conf", target: "/etc/app/config.yaml", location: "service web"},
		{source: "secret:api-key", target: "/etc/app/secret.key", location: "service web"},
		{source: "volume:./data", target: "/app/data", location: "service web"},
	}

	errs, warns := checkTargetConflict("web", mappings)

	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %v", len(errs), errs)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warns), warns)
	}
}

// TestCheckTargetConflict_MultipleConfigsConflict tests multiple configs targeting same path
func TestCheckTargetConflict_MultipleConfigsConflict(t *testing.T) {
	mappings := []targetMapping{
		{source: "config:conf1", target: "/etc/app.conf", location: "service web"},
		{source: "config:conf2", target: "/etc/app.conf", location: "service web"},
		{source: "config:conf3", target: "/etc/app.conf", location: "service web"},
	}

	errs, warns := checkTargetConflict("web", mappings)

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "duplicate configs/secrets") {
		t.Errorf("expected duplicate error, got: %s", errs[0])
	}
	// Should list all three sources in the error
	if !strings.Contains(errs[0], "conf1") || !strings.Contains(errs[0], "conf2") || !strings.Contains(errs[0], "conf3") {
		t.Errorf("error should mention all conflicting sources: %s", errs[0])
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warns), warns)
	}
}

// TestConvert_ConfigDefaultTarget tests that configs without target use default /<configName>
func TestConvert_ConfigDefaultTarget(t *testing.T) {
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}

	// Create temporary config file in current directory (relative path)
	cfgFile := "test_app.conf"
	if err := os.WriteFile(cfgFile, []byte("key=value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cfgFile)

	compose := "configs:\n  myconfig:\n    file: " + cfgFile + "\n\nservices:\n  web:\n    image: nginx\n    configs:\n      - myconfig\n"

	app := &model.App{
		Name:    "testapp",
		Compose: compose,
		Volumes: []model.AppVolume{{Name: "data", Size: 1}},
	}

	c := NewConverter(svc, prv, cls, app, "app")
	_, err := c.Convert(context.Background())
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	// Check that volumeMount uses default target: /myconfig
	if len(c.K8sContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(c.K8sContainers))
	}

	found := false
	for _, vm := range c.K8sContainers[0].VolumeMounts {
		if vm.MountPath == "/myconfig" && vm.ReadOnly {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected volumeMount with MountPath=/myconfig, got: %+v", c.K8sContainers[0].VolumeMounts)
	}
}

// TestConvert_SecretDefaultTarget tests that secrets without target use default /run/secrets/<secretName>
func TestConvert_SecretDefaultTarget(t *testing.T) {
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}

	// Create temporary secret file in current directory (relative path)
	secFile := "test_api.key"
	if err := os.WriteFile(secFile, []byte("secret123"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(secFile)

	compose := "secrets:\n  mysecret:\n    file: " + secFile + "\n\nservices:\n  web:\n    image: nginx\n    secrets:\n      - mysecret\n"

	app := &model.App{
		Name:    "testapp",
		Compose: compose,
		Volumes: []model.AppVolume{{Name: "data", Size: 1}},
	}

	c := NewConverter(svc, prv, cls, app, "app")
	_, err := c.Convert(context.Background())
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	// Check that volumeMount uses default target: /run/secrets/mysecret
	if len(c.K8sContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(c.K8sContainers))
	}

	found := false
	for _, vm := range c.K8sContainers[0].VolumeMounts {
		if vm.MountPath == "/run/secrets/mysecret" && vm.ReadOnly {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected volumeMount with MountPath=/run/secrets/mysecret, got: %+v", c.K8sContainers[0].VolumeMounts)
	}
}

// TestConvert_ConfigSecretExplicitTarget tests that explicit targets override defaults
func TestConvert_ConfigSecretExplicitTarget(t *testing.T) {
	svc := &model.Workspace{Name: "testsvc"}
	prv := &model.Provider{Name: "testprv", Driver: "test"}
	cls := &model.Cluster{Name: "testcls"}

	cfgFile := "test_app2.conf"
	secFile := "test_api2.key"
	if err := os.WriteFile(cfgFile, []byte("key=value\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cfgFile)
	if err := os.WriteFile(secFile, []byte("secret123"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(secFile)

	compose := "configs:\n  myconfig:\n    file: " + cfgFile + "\n\nsecrets:\n  mysecret:\n    file: " + secFile + "\n\nservices:\n  web:\n    image: nginx\n    configs:\n      - source: myconfig\n        target: /etc/app/config.conf\n    secrets:\n      - source: mysecret\n        target: /etc/app/secret.key\n"

	app := &model.App{
		Name:    "testapp",
		Compose: compose,
		Volumes: []model.AppVolume{{Name: "data", Size: 1}},
	}

	c := NewConverter(svc, prv, cls, app, "app")
	_, err := c.Convert(context.Background())
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	if len(c.K8sContainers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(c.K8sContainers))
	}

	// Check explicit config target
	foundCfg := false
	for _, vm := range c.K8sContainers[0].VolumeMounts {
		if vm.MountPath == "/etc/app/config.conf" && vm.ReadOnly {
			foundCfg = true
			break
		}
	}
	if !foundCfg {
		t.Errorf("expected config volumeMount at /etc/app/config.conf")
	}

	// Check explicit secret target
	foundSec := false
	for _, vm := range c.K8sContainers[0].VolumeMounts {
		if vm.MountPath == "/etc/app/secret.key" && vm.ReadOnly {
			foundSec = true
			break
		}
	}
	if !foundSec {
		t.Errorf("expected secret volumeMount at /etc/app/secret.key")
	}
}
