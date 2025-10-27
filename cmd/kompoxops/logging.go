package main

import (
	"context"
	"time"

	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/naming"
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
// - Start:   CMD:<operation>/S (with runId, resourceId in logger attributes)
// - Success: CMD:<operation>/EOK (with err, elapsed in logger attributes)
// - Failure: CMD:<operation>/EFAIL (with err, elapsed in logger attributes)
//
// All logs use INFO level (mechanical recording).
// See design/v1/Kompox-Logging.ja.md for the full Span pattern specification.
func withCmdRunLogger(ctx context.Context, operation, resourceID string) (context.Context, func(err error)) {
	runID, err := naming.NewCompactID()
	if err != nil {
		// Fallback to a fixed value if ID generation fails
		runID = "error"
	}

	startAt := time.Now()

	// Attach runId, resourceId to logger and return new context
	logger := logging.FromContext(ctx).With("runId", runID, "resourceId", resourceID)
	ctx = logging.WithLogger(ctx, logger)

	// Emit start log line
	logger.Info(ctx, "CMD:"+operation+"/S")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string
		if err == nil {
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
		logger.Info(ctx, msg, "err", errStr, "elapsed", elapsed)
	}

	return ctx, cleanup
}
