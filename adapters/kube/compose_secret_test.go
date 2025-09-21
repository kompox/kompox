package kube

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"k8s.io/utils/ptr"
)

// helper to create temp file with content and relative path
func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	fp := filepath.Join(dir, name)
	if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return name // relative path to base dir
}

func TestBuildComposeSecret_DotEnvBasicAndOverrides(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTemp(t, dir, "base.env", "A=1\nB=2\n# comment\nC=raw value\n")
	f2 := writeTemp(t, dir, "override.env", "B=22\nC=' spaced '\nD=4\n")

	// compose EnvFiles slice
	efs := []types.EnvFile{{Path: f1}, {Path: f2}}
	// service.Environment overrides (remove from secret)
	envOverrides := map[string]*string{"D": ptr.To("from_env")}

	labels := map[string]string{"test": "true"}
	sec, hash, err := buildComposeSecret(dir, "app", "app", "svc", efs, envOverrides, labels, "ns")
	if err != nil {
		t.Fatalf("buildComposeSecret: %v", err)
	}
	if sec == nil {
		t.Fatalf("expected secret")
	}
	if sec.Name != "app-app-svc" {
		t.Errorf("unexpected secret name %s", sec.Name)
	}
	if hash == "" {
		t.Errorf("expected non-empty hash")
	}
	if _, ok := sec.Data["D"]; ok {
		t.Errorf("key D should be removed due to env override")
	}
	// Expect keys: A,B,C (B overridden), no D
	wantKeys := []string{"A", "B", "C"}
	var gotKeys []string
	for k := range sec.Data {
		gotKeys = append(gotKeys, k)
	}
	slices.Sort(gotKeys)
	if !slices.Equal(gotKeys, wantKeys) {
		t.Errorf("keys mismatch got=%v want=%v", gotKeys, wantKeys)
	}
}

func TestBuildComposeSecret_JSON_YAML_Types(t *testing.T) {
	dir := t.TempDir()
	jf := writeTemp(t, dir, "vars.json", "{\n  \"A\": 10, \n  \"B\": true, \n  \"C\": false, \n  \"S\": \"str\"\n}\n")
	yf := writeTemp(t, dir, "vars.yaml", "A: 1\nB: true\nC: false\nS: str\n")

	efs := []types.EnvFile{{Path: jf}, {Path: yf}}
	sec, _, err := buildComposeSecret(dir, "app", "app", "svc", efs, map[string]*string{}, map[string]string{}, "ns")
	if err != nil {
		t.Fatalf("buildComposeSecret: %v", err)
	}
	// JSON overrides YAML (later file overrides earlier)
	// keys should all be strings now
	if string(sec.Data["A"]) != "1" && string(sec.Data["A"]) != "10" {
		t.Errorf("A should be stringified, got %q", string(sec.Data["A"]))
	}
	if string(sec.Data["B"]) != "true" {
		t.Errorf("B should be true, got %q", string(sec.Data["B"]))
	}
	if string(sec.Data["C"]) != "false" {
		t.Errorf("C should be false, got %q", string(sec.Data["C"]))
	}
	if string(sec.Data["S"]) != "str" {
		t.Errorf("S should be str, got %q", string(sec.Data["S"]))
	}
}

func TestBuildComposeSecret_Empty_NoEnvFiles(t *testing.T) {
	sec, hash, err := buildComposeSecret("/tmp", "app", "app", "svc", nil, map[string]*string{}, map[string]string{}, "ns")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sec != nil || hash != "" {
		t.Errorf("expected no secret and empty hash for nil envfiles")
	}
}

func TestReadEnvFile_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	bad := writeTemp(t, dir, "bad.env", "1ABC=zzz\n")
	_, err := readEnvFile(dir, bad)
	if err == nil {
		t.Fatalf("expected error for invalid key")
	}
}
