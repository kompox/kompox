package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kompox/kompox/internal/logging"
)

func TestRootExecuteC_MarksCMDStartLogEmittedOnSuccessfulCommand(t *testing.T) {
	t.Setenv("KOMPOX_ROOT", "")
	t.Setenv("KOMPOX_DIR", "")
	t.Setenv("KOMPOX_KOM_PATH", "")
	t.Setenv("KOMPOX_KOM_APP", "")

	tmpDir := t.TempDir()
	kompoxDir := filepath.Join(tmpDir, ".kompox")
	if err := os.MkdirAll(kompoxDir, 0o755); err != nil {
		t.Fatalf("creating .kompox directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kompoxDir, "config.yml"), []byte("version: 1\nstore:\n  type: local\n"), 0o644); err != nil {
		t.Fatalf("writing config.yml: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("restoring working directory: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("changing to temp directory: %v", err)
	}

	root := newRootCmd()
	root.SetContext(context.Background())
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"version"})

	executed, err := root.ExecuteC()
	if err != nil {
		t.Fatalf("ExecuteC() error = %v", err)
	}

	ctx := root.Context()
	if executed != nil {
		ctx = executed.Context()
	}
	if !hasEmittedCMDStartLog(ctx) {
		t.Fatal("expected CMD start log marker for successful command")
	}
	if getLogFilePath(ctx) == "" {
		t.Fatal("expected log file path to be recorded for successful command")
	}
	if closer, ok := ctx.Value(logFileCloserKey).(*logging.LogFile); ok && closer != nil {
		_ = closer.Close()
	}
}

func TestRootExecuteC_DoesNotMarkCMDStartLogEmittedOnFlagParseError(t *testing.T) {
	root := newRootCmd()
	root.SetContext(context.Background())
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"app", "deploy", "-v"})

	executed, err := root.ExecuteC()
	if err == nil {
		t.Fatal("expected flag parse error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown shorthand flag") {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := root.Context()
	if executed != nil {
		ctx = executed.Context()
	}
	if hasEmittedCMDStartLog(ctx) {
		t.Fatal("did not expect CMD start log marker for flag parse error")
	}
	if got := getLogFilePath(ctx); got != "" {
		t.Fatalf("expected no log file path for flag parse error, got %q", got)
	}
}
