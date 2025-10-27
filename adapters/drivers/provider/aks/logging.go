package aks

import (
	"context"
	"time"

	"github.com/kompox/kompox/internal/logging"
)

// withMethodLogger implements the Span pattern for AKS driver logging.
// It emits a start log line and returns a context with logger attributes attached,
// plus a cleanup function to emit the success or failure log line.
//
// Usage:
//
//	ctx, cleanup := d.withMethodLogger(ctx, "ClusterProvision")
//	defer func() { cleanup(err) }()
//
// Log message format:
// - Start:   AKS:<method>/S (with driver in logger attributes)
// - Success: AKS:<method>/EOK (with err, elapsed in logger attributes)
// - Failure: AKS:<method>/EFAIL (with err, elapsed in logger attributes)
//
// All logs use INFO level (mechanical recording).
// See design/v1/Kompox-Logging.ja.md for the full Span pattern specification.
func (d *driver) withMethodLogger(ctx context.Context, method string) (context.Context, func(err error)) {
	startAt := time.Now()

	// Attach driver=AKS.<method> to logger and return new context
	driverName := "AKS." + method
	logger := logging.FromContext(ctx).With("driver", driverName)
	ctx = logging.WithLogger(ctx, logger)

	// Emit start log line
	logger.Info(ctx, "AKS:"+method+"/S")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string
		if err == nil {
			msg = "AKS:" + method + "/EOK"
			errStr = ""
		} else {
			msg = "AKS:" + method + "/EFAIL"
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
