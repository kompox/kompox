package main

import (
	"context"
	"errors"
	"time"

	"github.com/kompox/kompox/internal/logging"
)

// withCmdRunLogger implements the Span pattern for CLI command logging.
// It emits a start log line and returns a context with logger attributes attached,
// plus a cleanup function to emit the success or failure log line.
//
// Usage:
//
//	ctx, cleanup := withCmdRunLogger(ctx, "cluster.provision", resourceID)
//	defer func() { cleanup(err) }()
//
// Log message format:
// - Start:   CMD:<operation>/S (with resourceId in logger attributes)
// - Success: CMD:<operation>/EOK (with err, elapsed in logger attributes)
// - Failure: CMD:<operation>/EFAIL (with err, elapsed in logger attributes)
//
// Note: ExitCodeError is treated as EOK since it represents subprocess exit code
// propagation rather than a command failure.
//
// All logs use INFO level (mechanical recording).
// The runId is inherited from the context logger (set in PersistentPreRunE).
// See design/v1/Kompox-Logging.ja.md for the full Span pattern specification.
func withCmdRunLogger(ctx context.Context, operation, resourceID string) (context.Context, func(err error)) {
	startAt := time.Now()

	// Attach resourceId to logger and return new context
	logger := logging.FromContext(ctx).With("resourceId", resourceID)
	ctx = logging.WithLogger(ctx, logger)

	// Emit start log line
	logger.Info(ctx, "CMD:"+operation+"/S")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string

		// ExitCodeError is not a command failure; it's subprocess exit code propagation
		var exitCodeErr ExitCodeError
		isExitCodeErr := errors.As(err, &exitCodeErr)

		if err == nil || isExitCodeErr {
			msg = "CMD:" + operation + "/EOK"
			errStr = ""
		} else {
			msg = "CMD:" + operation + "/EFAIL"
			errMsg := err.Error()
			if len(errMsg) > 32 {
				errStr = errMsg[:32] + "..."
			} else {
				errStr = errMsg
			}
		}

		// Always use INFO level for Span pattern (mechanical recording)
		if isExitCodeErr {
			logger.Info(ctx, msg, "err", errStr, "exitCode", exitCodeErr.Code, "elapsed", elapsed)
		} else {
			logger.Info(ctx, msg, "err", errStr, "elapsed", elapsed)
		}
	}

	return ctx, cleanup
}
