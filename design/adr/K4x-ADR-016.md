---
id: K4x-ADR-016
title: CLI Logging Strategy
status: accepted
date: 2025-12-13
language: en
supersedes: []
supersededBy: []
---
# K4x-ADR-016: CLI Logging Strategy

## Context

- The current CLI outputs INFO-level logs to stderr, making it difficult to distinguish operational traces from actionable warnings and errors.
- Structured logging patterns (Event/Span/Step) defined in [Kompox-Logging.ja.md] are valuable for debugging but create noise in interactive CLI use.
- Users need operation traces for post-hoc debugging, but these should not clutter the console during normal use.
- [K4x-ADR-015] established `.kompox/` as the project environment directory and mentioned future extensions for logs, cache, and temporary files.

## Decision

Implement a file-based logging strategy that captures structured operation logs without cluttering console output.

### Structured Log Output

By default, structured logs are written to a file in `$KOMPOX_DIR/logs/`. No structured log output to console unless explicitly configured.

Log output is controlled by the following flags:

| Flag | Description |
|------|-------------|
| `--log-format <FORMAT>` | Log format: `json` (default) or `human` |
| `--log-level <LEVEL>` | Minimum log level: `DEBUG`, `INFO` (default), `WARN`, `ERROR` |
| `--log-output <PATH>` | Log output destination (see below) |

`--log-output` values:

| Value | Behavior |
|-------|----------|
| (empty/omitted) | Write to `$KOMPOX_LOG_DIR/kompoxops-YYYYMMDD-HHMMSS-sss.log` |
| `<path>` | Write to specified path (absolute or relative to `$KOMPOX_LOG_DIR`) |
| `-` | Write to stderr |
| `none` | Disable log output entirely |

Note: `-v`, `--verbose`, and `--debug` flags do not affect structured log output. These flags control stdout/stderr content and are interpreted independently by each command.

### Error Exit Behavior

On abnormal termination, the CLI outputs a human-readable error message to stderr that includes the full path to the log file:

```
Error: deployment failed: manifest validation error
See log file for details: /path/to/project/.kompox/logs/kompoxops-20251213-095105-123.log
```

This allows users to locate detailed traces without enabling verbose mode.

### File Output

Automatic file logging captures operation traces for debugging:

| Aspect | Specification |
|--------|---------------|
| Default location | `$KOMPOX_DIR/logs/` |
| Format | JSON Lines (machine-readable) |
| Level | INFO and above |
| File naming | `kompoxops-YYYYMMDD-HHMMSS-sss.log` |
| Timezone | UTC |
| Retention | 7-day default |

File naming example: `kompoxops-20251213-095105-123.log` (December 13, 2025, 09:51:05.123 UTC)

Each CLI invocation creates a new log file. This ensures parallel executions do not interfere with each other.

### Configuration

Environment variables:

| Variable | Description |
|----------|-------------|
| `KOMPOX_LOG_DIR` | Override log directory (default: `$KOMPOX_DIR/logs`) |
| `KOMPOX_LOG_FORMAT` | Override log format (default: `json`) |
| `KOMPOX_LOG_LEVEL` | Override log level (default: `INFO`) |
| `KOMPOX_LOG_OUTPUT` | Override log output destination |

File logging configuration in `.kompox/config.yml`:

```yaml
version: 1
logging:
  dir: $KOMPOX_DIR/logs   # default
  format: json            # default
  level: INFO             # default
  retentionDays: 7        # default
```

### Integration with kompoxops init

The `kompoxops init` command:

1. Creates `.kompox/logs/` directory
2. Adds `logs/` entry to `.kompox/.gitignore`

### Log Content by Level

| Pattern | DEBUG | INFO | WARN | ERROR |
|---------|-------|------|------|-------|
| Event (ERROR) | ✓ | ✓ | ✓ | ✓ |
| Event (WARN) | ✓ | ✓ | ✓ | |
| Event (INFO) | ✓ | ✓ | | |
| Span (`/S`, `/EOK`, `/EFAIL`) | ✓ | ✓ | | |
| Step (`/s`, `/eok`, `/efail`) | ✓ | ✓ | | |
| DEBUG events | ✓ | | | |

### Log File Retention

- Files older than retention period are deleted on CLI startup
- Each invocation creates a unique file; no rotation within a single run
- Parallel invocations are safe due to unique timestamps with millisecond precision

## Alternatives Considered

- **Use `-v`/`--verbose`/`--debug` for log control**: Rejected; these flags should control command-specific stdout/stderr output, not structured logging. Separating concerns allows independent control.
- **Output WARN/ERROR to console by default**: Rejected; JSON-formatted log messages on stderr are confusing for interactive use. Error messages should be human-readable, with log file path for details.
- **Keep all INFO on stderr**: Rejected; too noisy for interactive use and makes error identification difficult.
- **Log to system log (syslog/journald)**: Rejected; not portable across platforms (Windows, macOS, Linux).
- **Log to user home directory (`~/.kompox/logs/`)**: Rejected; logs should be project-scoped for isolation and easier cleanup.
- **Single log file per day with append**: Rejected; parallel invocations could interleave log entries, making debugging difficult.
- **Use PID in filename instead of timestamp**: Rejected; timestamps are more meaningful for temporal correlation across sessions.
- **`--no-log-file` flag**: Rejected; `--log-output none` provides the same functionality with a more consistent interface.

## Consequences

### Pros

- Clean console output: no structured logs by default, only human-readable error messages
- Error messages include log file path for easy access to detailed traces
- Full operation trace available in files for post-hoc debugging
- Parallel-safe: unique log file per invocation prevents interleaving
- Git-ignored logs prevent accidental commits of potentially sensitive operation details
- Consistent with `.kompox/` directory design from [K4x-ADR-015]
- Configurable for CI/CD environments (disable file logging, increase verbosity)

### Cons

- Disk usage for log files (mitigated by 7-day default retention)
- Many small log files instead of consolidated daily logs
- Users must use `--log-output -` to see structured logs on console

## References

- [K4x-ADR-015]: Kompox CLI Env with `.kompox/` directory
- [Kompox-Logging.ja.md]: Structured logging patterns specification
- [Kompox-CLI.ja.md]: CLI specification

[K4x-ADR-015]: ./K4x-ADR-015.md
[Kompox-Logging.ja.md]: ../v1/Kompox-Logging.ja.md
[Kompox-CLI.ja.md]: ../v1/Kompox-CLI.ja.md
