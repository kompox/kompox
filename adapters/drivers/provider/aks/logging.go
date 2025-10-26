package aks

import (
	"context"
	"time"

	"github.com/kompox/kompox/internal/logging"
)

// withMethodLogger implements the Span pattern for AKS driver logging.
// It emits a START log line and returns a context with logger attributes attached,
// plus a cleanup function to emit the END:OK or END:FAILED log line.
//
// Usage:
//
//	ctx, cleanup := d.withMethodLogger(ctx, "ClusterProvision")
//	defer func() { cleanup(err) }()
//
// Log message format:
// - START:  AKS:<method>:START (with driver in logger attributes)
// - END:    AKS:<method>:END:OK or AKS:<method>:END:FAILED (with err, elapsed in logger attributes)
//
// See design/v1/Kompox-Logging.ja.md for the full Span pattern specification.
func (d *driver) withMethodLogger(ctx context.Context, method string) (context.Context, func(err error)) {
	startAt := time.Now()

	// Attach driver=AKS.<method> to logger and return new context
	driverName := "AKS." + method
	logger := logging.FromContext(ctx).With("driver", driverName)
	ctx = logging.WithLogger(ctx, logger)

	// Emit START log line
	logger.Info(ctx, "AKS:"+method+":START")

	cleanup := func(err error) {
		elapsed := time.Since(startAt).Seconds()
		var msg, errStr string
		if err == nil {
			msg = "AKS:" + method + ":END:OK"
			errStr = ""
		} else {
			msg = "AKS:" + method + ":END:FAILED"
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
