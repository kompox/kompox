package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateLogFilename(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "basic timestamp",
			time:     time.Date(2025, 12, 13, 9, 51, 5, 123000000, time.UTC),
			expected: "kompoxops-20251213-095105-123.log",
		},
		{
			name:     "midnight",
			time:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "kompoxops-20250101-000000-000.log",
		},
		{
			name:     "end of day",
			time:     time.Date(2025, 12, 31, 23, 59, 59, 999000000, time.UTC),
			expected: "kompoxops-20251231-235959-999.log",
		},
		{
			name:     "milliseconds precision",
			time:     time.Date(2025, 6, 15, 12, 30, 45, 456789000, time.UTC),
			expected: "kompoxops-20250615-123045-456.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateLogFilename(tt.time)
			if result != tt.expected {
				t.Errorf("GenerateLogFilename() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNewLogFile_None(t *testing.T) {
	cfg := &LogConfig{
		Output: "none",
		Dir:    t.TempDir(),
	}

	lf, err := NewLogFile(cfg)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	if lf.Path != "" {
		t.Errorf("Path should be empty for 'none' output, got %q", lf.Path)
	}
	if lf.Writer() == nil {
		t.Error("Writer should not be nil")
	}
}

func TestNewLogFile_Stderr(t *testing.T) {
	cfg := &LogConfig{
		Output: "-",
		Dir:    t.TempDir(),
	}

	lf, err := NewLogFile(cfg)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	if lf.Path != "" {
		t.Errorf("Path should be empty for '-' output, got %q", lf.Path)
	}
	if lf.Writer() != os.Stderr {
		t.Error("Writer should be os.Stderr")
	}
}

func TestNewLogFile_AutoGenerate(t *testing.T) {
	dir := t.TempDir()
	cfg := &LogConfig{
		Output: "",
		Dir:    dir,
	}

	lf, err := NewLogFile(cfg)
	if err != nil {
		t.Fatalf("NewLogFile() error = %v", err)
	}
	defer lf.Close()

	if lf.Path == "" {
		t.Error("Path should not be empty for auto-generated output")
	}
	if filepath.Dir(lf.Path) != dir {
		t.Errorf("Path should be in dir %q, got %q", dir, filepath.Dir(lf.Path))
	}

	// Verify file was created
	if _, err := os.Stat(lf.Path); os.IsNotExist(err) {
		t.Errorf("Log file was not created: %s", lf.Path)
	}
}

func TestNewLogFile_SpecifiedPath(t *testing.T) {
	dir := t.TempDir()

	t.Run("absolute path", func(t *testing.T) {
		absPath := filepath.Join(dir, "custom.log")
		cfg := &LogConfig{
			Output: absPath,
			Dir:    dir,
		}

		lf, err := NewLogFile(cfg)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}
		defer lf.Close()

		if lf.Path != absPath {
			t.Errorf("Path = %q, want %q", lf.Path, absPath)
		}
	})

	t.Run("relative path", func(t *testing.T) {
		cfg := &LogConfig{
			Output: "relative.log",
			Dir:    dir,
		}

		lf, err := NewLogFile(cfg)
		if err != nil {
			t.Fatalf("NewLogFile() error = %v", err)
		}
		defer lf.Close()

		expectedPath := filepath.Join(dir, "relative.log")
		if lf.Path != expectedPath {
			t.Errorf("Path = %q, want %q", lf.Path, expectedPath)
		}
	})
}

func TestCleanupOldLogFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files with different modification times
	oldTime := time.Now().AddDate(0, 0, -10) // 10 days ago
	newTime := time.Now().AddDate(0, 0, -3)  // 3 days ago

	// Old file (should be deleted)
	oldFile := filepath.Join(dir, "kompoxops-20251201-120000-000.log")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// New file (should be kept)
	newFile := filepath.Join(dir, "kompoxops-20251210-120000-000.log")
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newFile, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	// Non-matching file (should be kept)
	otherFile := filepath.Join(dir, "other.log")
	if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(otherFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Run cleanup with 7-day retention
	if err := CleanupOldLogFiles(dir, 7); err != nil {
		t.Fatalf("CleanupOldLogFiles() error = %v", err)
	}

	// Check old file was deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("Old log file should have been deleted: %s", oldFile)
	}

	// Check new file was kept
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Errorf("New log file should have been kept: %s", newFile)
	}

	// Check non-matching file was kept
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Errorf("Non-matching file should have been kept: %s", otherFile)
	}
}

func TestCleanupOldLogFiles_NonExistentDir(t *testing.T) {
	// Should not error for non-existent directory
	err := CleanupOldLogFiles("/nonexistent/path", 7)
	if err != nil {
		t.Errorf("CleanupOldLogFiles() should not error for non-existent dir, got: %v", err)
	}
}

func TestCleanupOldLogFiles_ZeroRetention(t *testing.T) {
	dir := t.TempDir()

	// Create a file
	file := filepath.Join(dir, "kompoxops-20251201-120000-000.log")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// With 0 retention, nothing should be deleted
	if err := CleanupOldLogFiles(dir, 0); err != nil {
		t.Fatalf("CleanupOldLogFiles() error = %v", err)
	}

	// File should still exist
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Errorf("File should have been kept with 0 retention: %s", file)
	}
}
