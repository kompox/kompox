package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogConfig holds configuration for structured log output.
type LogConfig struct {
	Format        string // "json" (default) or "human"
	Level         string // "DEBUG", "INFO" (default), "WARN", "ERROR"
	Output        string // Path, "-" for stderr, "none" to disable
	Dir           string // Log directory (default: $KOMPOX_DIR/logs)
	RetentionDays int    // Days to retain log files (default: 7)
}

// LogFile manages a log file lifecycle.
type LogFile struct {
	Path   string   // Full path to the log file (empty if output is disabled)
	file   *os.File // Opened file handle (nil if stderr or disabled)
	writer io.Writer
}

// NewLogFile creates a new log file based on the configuration.
// Returns the LogFile struct with writer set appropriately.
// When writing to a file, the command line is recorded as the first log entry.
//
// Output behavior:
//   - empty/omitted: Create auto-generated file in Dir
//   - "-": Use os.Stderr
//   - "none": Disable logging (io.Discard)
//   - path: Use specified path (absolute or relative to Dir)
func NewLogFile(cfg *LogConfig) (*LogFile, error) {
	lf := &LogFile{}

	switch strings.ToLower(cfg.Output) {
	case "none":
		// Disable logging
		lf.writer = io.Discard
		return lf, nil

	case "-":
		// Output to stderr
		lf.writer = os.Stderr
		return lf, nil

	case "":
		// Auto-generate filename
		filename := GenerateLogFilename(time.Now().UTC())
		lf.Path = filepath.Join(cfg.Dir, filename)

	default:
		// Specified path - absolute or relative to Dir
		if filepath.IsAbs(cfg.Output) {
			lf.Path = cfg.Output
		} else {
			lf.Path = filepath.Join(cfg.Dir, cfg.Output)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(lf.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory %q: %w", dir, err)
	}

	// Open file for writing
	f, err := os.OpenFile(lf.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file %q: %w", lf.Path, err)
	}

	lf.file = f
	lf.writer = f

	return lf, nil
}

// Writer returns the io.Writer for log output.
func (lf *LogFile) Writer() io.Writer {
	return lf.writer
}

// Close closes the log file if it was opened.
func (lf *LogFile) Close() error {
	if lf.file != nil {
		return lf.file.Close()
	}
	return nil
}

// GenerateLogFilename generates a log filename with format:
// kompoxops-YYYYMMDD-HHMMSS-sss.log
// where sss is milliseconds. Uses UTC timezone.
func GenerateLogFilename(t time.Time) string {
	return fmt.Sprintf("kompoxops-%s-%03d.log",
		t.Format("20060102-150405"),
		t.Nanosecond()/1_000_000)
}

// CleanupOldLogFiles removes log files older than retentionDays from the directory.
// It only deletes files matching the pattern "kompoxops-*.log".
func CleanupOldLogFiles(dir string, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, nothing to clean
		}
		return fmt.Errorf("reading log directory %q: %w", dir, err)
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "kompoxops-") || !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil {
				// Log removal failures but don't fail the operation
				continue
			}
		}
	}

	return nil
}
