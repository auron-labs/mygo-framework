// Package desktop maps application services to concrete MyGo wire responses.
package desktop

import (
	"context"
	"log/slog"
	"time"

	"github.com/auron-labs/mygo-framework/logging"
	"github.com/auron-labs/mygo-framework/problem"
)

func desktopProblem(
	ctx context.Context,
	logger *slog.Logger,
	operation string,
	startedAt time.Time,
	err error,
) *problem.Problem {
	mapped := problem.FromError(err)
	if mapped.Code == problem.CodeInternal && logger != nil {
		logging.ReportUnexpected(ctx, logger, operation, "desktop", err,
			slog.Duration("duration", time.Since(startedAt)),
		)
	}

	return mapped
}
