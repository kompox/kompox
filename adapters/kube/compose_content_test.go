package kube

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/kompox/kompox/internal/naming"
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

func TestMergeEnvFiles_DotEnvBasic(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTemp(t, dir, "base.env", "A=1\nB=2\n# comment\nC=raw value\n")
	f2 := writeTemp(t, dir, "override.env", "B=22\nC=' spaced '\nD=4\n")
	envFiles := []types.EnvFile{
		{Path: f1, Required: true},
		{Path: f2, Required: true},
	}
	kv, overrides, err := mergeEnvFiles(dir, envFiles)
	if err != nil {
		t.Fatalf("mergeEnvFiles: %v", err)
	}
	if len(overrides) == 0 {
		t.Errorf("expected overrides map to record at least one previous value")
	}
	// Keys after merge contain all (A,B,C,D)
	wantKeys := []string{"A", "B", "C", "D"}
	var got []string
	for k := range kv {
		got = append(got, k)
	}
	slices.Sort(got)
	if !slices.Equal(got, wantKeys) {
		t.Errorf("keys mismatch got=%v want=%v", got, wantKeys)
	}
	// Hash deterministic
	h := ComputeContentHash(kv)
	if h == "" {
		t.Errorf("expected non-empty hash")
	}
}

func TestMergeEnvFiles_JSON_YAML_Types(t *testing.T) {
	dir := t.TempDir()
	jf := writeTemp(t, dir, "vars.json", "{\n  \"A\": 10, \n  \"B\": true, \n  \"C\": false, \n  \"S\": \"str\"\n}\n")
	yf := writeTemp(t, dir, "vars.yaml", "A: 1\nB: true\nC: false\nS: str\n")
	envFiles := []types.EnvFile{
		{Path: jf, Required: true},
		{Path: yf, Required: true},
	}
	kv, _, err := mergeEnvFiles(dir, envFiles)
	if err != nil {
		t.Fatalf("mergeEnvFiles: %v", err)
	}
	if kv["B"] != "true" {
		t.Errorf("B should be true got=%s", kv["B"])
	}
	if kv["C"] != "false" {
		t.Errorf("C should be false got=%s", kv["C"])
	}
	if kv["S"] != "str" {
		t.Errorf("S should be str got=%s", kv["S"])
	}
	if kv["A"] == "" {
		t.Errorf("A should be present")
	}
}

func TestMergeEnvFiles_Empty(t *testing.T) {
	kv, _, err := mergeEnvFiles(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("mergeEnvFiles: %v", err)
	}
	if len(kv) != 0 {
		t.Errorf("expected empty map")
	}
	if ComputeContentHash(kv) != naming.ShortHash("", 6) {
		t.Errorf("empty hash mismatch")
	}
}

func TestReadEnvDirFile_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	bad := writeTemp(t, dir, "bad.env", "1ABC=zzz\n")
	_, err := ReadEnvDirFile(dir, bad, true)
	if err == nil {
		t.Fatalf("expected error for invalid key")
	}
}

func TestReadEnvDirFile_OptionalMissing(t *testing.T) {
	dir := t.TempDir()
	// File does not exist, but required=false should not error
	kv, err := ReadEnvDirFile(dir, "nonexistent.env", false)
	if err != nil {
		t.Fatalf("expected no error for optional missing file, got: %v", err)
	}
	if len(kv) != 0 {
		t.Errorf("expected empty map for optional missing file, got: %v", kv)
	}
}

func TestReadEnvDirFile_RequiredMissing(t *testing.T) {
	dir := t.TempDir()
	// File does not exist and required=true should error
	_, err := ReadEnvDirFile(dir, "nonexistent.env", true)
	if err == nil {
		t.Fatalf("expected error for required missing file")
	}
}

func TestMergeEnvFiles_OptionalMissing(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTemp(t, dir, "base.env", "A=1\nB=2\n")
	envFiles := []types.EnvFile{
		{Path: f1, Required: true},
		{Path: "missing.env", Required: false}, // Optional file that doesn't exist
	}
	kv, _, err := mergeEnvFiles(dir, envFiles)
	if err != nil {
		t.Fatalf("mergeEnvFiles: %v", err)
	}
	// Should only contain keys from base.env
	if kv["A"] != "1" || kv["B"] != "2" {
		t.Errorf("expected A=1 B=2, got: %v", kv)
	}
	if len(kv) != 2 {
		t.Errorf("expected 2 keys, got: %d", len(kv))
	}
}
