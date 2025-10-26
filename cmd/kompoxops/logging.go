package main

import (
	"context"
	"time"

	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/naming"
)

// withCmdRunLogger implements the Span pattern for CLI command logging.
// It emits a START log line and returns a context with logger attributes attached,
// plus a cleanup function to emit the END:OK or END:FAILED log line.
//
// Usage:
//
//	ctx, cleanup := withCmdRunLogger(ctx, "cluster.provision", resourceID)
//	defer func() { cleanup(err) }()
//
// Log message format:
// - START:  CMD:<operation>:START (with runId, resourceId in logger attributes)
// - END:    CMD:<operation>:END:OK or CMD:<operation>:END:FAILED (with err, elapsed in logger attributes)
//
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

	// Emit header line
	logger.Info(ctx, "CMD:"+operation+":START")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string
		if err == nil {
			msg = "CMD:" + operation + ":END:OK"
			errStr = ""
		} else {
			msg = "CMD:" + operation + ":END:FAILED"
			errMsg := err.Error()
			if len(errMsg) > 32 {
				errStr = errMsg[:32] + "..."
			} else {
				errStr = errMsg
			}
		}

		if err == nil {
			logger.Info(ctx, msg, "err", errStr, "elapsed", elapsed)
		} else {
			logger.Warn(ctx, msg, "err", errStr, "elapsed", elapsed)
		}
	}

	return ctx, cleanup
}
