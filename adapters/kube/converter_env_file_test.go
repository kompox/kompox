package kube

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/kompox/kompox/domain/model"
)

// TestEnvFileSingleService verifies Secret generation and envFrom reference for one service.
func TestEnvFileSingleService(t *testing.T) {
	ctx := context.Background()
	compose := "services:\n" +
		"  web:\n" +
		"    image: busybox:1.36\n" +
		"    env_file:\n" +
		"      - a.env\n" +
		"      - b.json\n" +
		"    environment:\n" +
		"      RUNTIME: x\n"
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}

	// Create temporary env files under current working dir (the converter passes baseDir '.')
	// We write them into a temp dir and chdir temporarily.
	tmp := t.TempDir()
	app := &model.App{Name: "demo", Compose: compose, RefBase: "file://" + tmp + "/"}
	c := NewConverter(svc, prv, cls, app, "app")

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.WriteFile("a.env", []byte("A=1\nB=2\n"), 0o644); err != nil {
		t.Fatalf("write a.env: %v", err)
	}
	if err := os.WriteFile("b.json", []byte(`{"B":22,"C":true}`), 0o644); err != nil {
		t.Fatalf("write b.json: %v", err)
	}

	if _, err := c.Convert(ctx); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(c.K8sSecrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(c.K8sSecrets))
	}
	sec := c.K8sSecrets[0]
	if sec.Name != "demo-app-web-base" {
		t.Errorf("unexpected secret name %s", sec.Name)
	}
	if sec.Data["A"] == nil || string(sec.Data["A"]) != "1" {
		t.Errorf("missing A=1 in secret")
	}
	if sec.Data["B"] == nil || string(sec.Data["B"]) != "22" {
		t.Errorf("expected B=22 override, got %q", sec.Data["B"])
	}
	if sec.Data["C"] == nil || string(sec.Data["C"]) != "true" {
		t.Errorf("expected C=true, got %q", sec.Data["C"])
	}
	// RUNTIME must not appear (environment override removed)
	if _, ok := sec.Data["RUNTIME"]; ok {
		t.Errorf("RUNTIME should be excluded from secret")
	}

	// Build to inspect pod annotation & envFrom
	// Need minimal volume binding even if no volumes (none defined so skip BindVolumes)
	if _, err := c.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if c.K8sDeployment == nil {
		t.Fatalf("deployment missing")
	}
	// Validate envFrom contains base and override optional secrets
	ctn0 := c.K8sDeployment.Spec.Template.Spec.Containers[0]
	if len(ctn0.EnvFrom) != 2 {
		t.Fatalf("expected 2 envFrom entries, got %d", len(ctn0.EnvFrom))
	}
	names := map[string]bool{}
	for _, ef := range ctn0.EnvFrom {
		if ef.SecretRef == nil {
			t.Errorf("envFrom without secretRef")
			continue
		}
		n := ef.SecretRef.Name
		names[n] = true
		if ef.SecretRef.Optional == nil || !*ef.SecretRef.Optional {
			t.Errorf("secret %s should be optional", n)
		}
	}
	for _, want := range []string{"demo-app-web-base", "demo-app-web-override"} {
		if !names[want] {
			t.Errorf("missing envFrom secret %s", want)
		}
	}
}

// TestEnvFileMultipleServices verifies multiple secrets order.
func TestEnvFileMultipleServices(t *testing.T) {
	ctx := context.Background()
	compose := "services:\n" +
		"  api:\n" +
		"    image: busybox:1.36\n" +
		"    env_file:\n" +
		"      - api.env\n" +
		"  worker:\n" +
		"    image: busybox:1.36\n" +
		"    env_file:\n" +
		"      - worker.env\n"
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}

	tmp := t.TempDir()
	app := &model.App{Name: "demo2", Compose: compose, RefBase: "file://" + tmp + "/"}
	c := NewConverter(svc, prv, cls, app, "app")

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if err := os.WriteFile("api.env", []byte("X=1\n"), 0o644); err != nil {
		t.Fatalf("write api.env: %v", err)
	}
	if err := os.WriteFile("worker.env", []byte("Y=2\n"), 0o644); err != nil {
		t.Fatalf("write worker.env: %v", err)
	}
	if _, err := c.Convert(ctx); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if len(c.K8sSecrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(c.K8sSecrets))
	}
	// Order of services iteration from compose-go may vary (map iteration). Validate set equality.
	seen := map[string]struct{}{}
	for _, s := range c.K8sSecrets {
		seen[s.Name] = struct{}{}
	}
	if _, ok := seen["demo2-app-api-base"]; !ok {
		t.Errorf("missing secret demo2-app-api")
	}
	if _, ok := seen["demo2-app-worker-base"]; !ok {
		t.Errorf("missing secret demo2-app-worker")
	}
	if _, err := c.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	// Ensure both containers reference their secrets
	if len(c.K8sDeployment.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers")
	}
	for _, ctr := range c.K8sDeployment.Spec.Template.Spec.Containers {
		if len(ctr.EnvFrom) != 2 {
			t.Errorf("container %s expected 2 envFrom entries, got %d", ctr.Name, len(ctr.EnvFrom))
		}
		wantBase := fmt.Sprintf("demo2-app-%s-base", ctr.Name)
		wantOverride := fmt.Sprintf("demo2-app-%s-override", ctr.Name)
		seenNames := map[string]bool{}
		for _, ef := range ctr.EnvFrom {
			if ef.SecretRef != nil {
				seenNames[ef.SecretRef.Name] = true
			}
		}
		if !seenNames[wantBase] {
			t.Errorf("container %s missing base secret %s", ctr.Name, wantBase)
		}
		if !seenNames[wantOverride] {
			t.Errorf("container %s missing override secret %s", ctr.Name, wantOverride)
		}
	}
}

// TestEnvFromWithoutEnvFile ensures even when no env_file is specified, base/override envFrom entries are present.
func TestEnvFromWithoutEnvFile(t *testing.T) {
	ctx := context.Background()
	compose := "services:\n" +
		"  lone:\n" +
		"    image: busybox:1.36\n"
	svc := &model.Workspace{Name: "svc"}
	prv := &model.Provider{Name: "prv", Driver: "test"}
	cls := &model.Cluster{Name: "cls"}
	app := &model.App{Name: "novars", Compose: compose}
	c := NewConverter(svc, prv, cls, app, "app")
	if _, err := c.Convert(ctx); err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if _, err := c.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(c.K8sDeployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container")
	}
	envFrom := c.K8sDeployment.Spec.Template.Spec.Containers[0].EnvFrom
	if len(envFrom) != 2 {
		t.Fatalf("expected 2 envFrom entries, got %d", len(envFrom))
	}
	names := map[string]bool{}
	for _, ef := range envFrom {
		if ef.SecretRef != nil {
			names[ef.SecretRef.Name] = true
			if ef.SecretRef.Optional == nil || !*ef.SecretRef.Optional {
				t.Errorf("secret %s should be optional", ef.SecretRef.Name)
			}
		}
	}
	for _, want := range []string{"novars-app-lone-base", "novars-app-lone-override"} {
		if !names[want] {
			t.Errorf("missing envFrom secret %s", want)
		}
	}
}
