package logging

import (
	"context"
	"errors"
	"log/slog"
)

// ReportUnexpected logs an unexpected failure at the adapter boundary.
func ReportUnexpected(
	ctx context.Context,
	logger *slog.Logger,
	operation string,
	component string,
	err error,
	attrs ...slog.Attr,
) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	fields := make([]slog.Attr, 0, len(attrs)+3)
	fields = append(fields,
		slog.String("operation", operation),
		slog.String("component", component),
		slog.Any("error", err),
	)
	fields = append(fields, attrs...)
	logger.LogAttrs(ctx, slog.LevelError, "operation failed", fields...)
}
