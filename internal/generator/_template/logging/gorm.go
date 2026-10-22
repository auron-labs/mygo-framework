package logging

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"
)

const slowQueryThreshold = 200 * time.Millisecond

// GORMHandler routes safe ORM diagnostics through the application logger.
type GORMHandler struct {
	logger *slog.Logger
	level  logger.LogLevel
}

// NewGORM constructs a GORM logger that never records SQL or parameter values.
func NewGORM(applicationLogger *slog.Logger) *GORMHandler {
	return &GORMHandler{logger: applicationLogger, level: logger.Warn}
}

// LogMode returns a copy configured with GORM's requested verbosity.
func (l *GORMHandler) LogMode(level logger.LogLevel) logger.Interface {
	updated := *l
	updated.level = level

	return &updated
}

// Info routes a safe generic ORM diagnostic without interpolation data.
func (l *GORMHandler) Info(ctx context.Context, _ string, _ ...any) {
	if l.level >= logger.Info {
		l.logger.DebugContext(ctx, "gorm diagnostic", slog.String("component", "gorm"))
	}
}

// Warn routes a safe generic ORM warning without interpolation data.
func (l *GORMHandler) Warn(ctx context.Context, _ string, _ ...any) {
	if l.level >= logger.Warn {
		l.logger.WarnContext(ctx, "gorm warning", slog.String("component", "gorm"))
	}
}

// Error leaves query failures for the transport boundary to report once.
func (*GORMHandler) Error(context.Context, string, ...any) {}

// Trace records duration and row count without evaluating or logging SQL text.
func (l *GORMHandler) Trace(
	ctx context.Context,
	begin time.Time,
	details func() (sql string, rowsAffected int64),
	err error,
) {
	if l.level <= logger.Silent || err != nil {
		return
	}

	duration := time.Since(begin)
	if duration <= slowQueryThreshold && l.level < logger.Info {
		return
	}
	_, rows := details()
	attrs := []any{
		slog.String("operation", "gorm.query"),
		slog.String("component", "gorm"),
		slog.Duration("duration", duration),
		slog.Int64("rows", rows),
	}
	if duration > slowQueryThreshold && l.level >= logger.Warn {
		l.logger.WarnContext(ctx, "slow database query", attrs...)
		return
	}
	if l.level >= logger.Info {
		l.logger.DebugContext(ctx, "database query completed", attrs...)
	}
}
