// Package desktop maps application services to concrete MyGo wire responses.
package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"mygo-framework/app/requests"
	"mygo-framework/app/services"
	"mygo-framework/logging"
	"mygo-framework/problem"
	appRuntime "mygo-framework/runtime"
	"time"
)

// CreateNoteResponse is the concrete desktop response for creating a note.
type CreateNoteResponse struct {
	Data    *services.CreateNoteOutput `json:"data"`
	Problem *problem.Problem           `json:"problem"`
}

// ListNotesResponse is the concrete desktop response for listing notes.
type ListNotesResponse struct {
	Data    *services.ListNotesOutput `json:"data"`
	Problem *problem.Problem          `json:"problem"`
}

// Notes exposes only note operations to MyGo.
type Notes struct {
	readiness *appRuntime.Readiness
	resolve   func() (*services.Notes, *slog.Logger, error)
}

// NewNotes constructs a desktop adapter without resolving runtime services.
func NewNotes(
	readiness *appRuntime.Readiness,
	resolve func() (*services.Notes, *slog.Logger, error),
) *Notes {
	return &Notes{readiness: readiness, resolve: resolve}
}

// Create creates a note through the shared application service.
func (a *Notes) Create(ctx context.Context, input requests.CreateNoteInput) CreateNoteResponse {
	startedAt := time.Now()
	service, logger, err := a.dependencies(ctx)
	if err != nil {
		return CreateNoteResponse{Problem: desktopProblem(ctx, logger, "notes.create", startedAt, err)}
	}

	output, err := service.Create(ctx, input)
	if err != nil {
		return CreateNoteResponse{Problem: desktopProblem(ctx, logger, "notes.create", startedAt, err)}
	}

	return CreateNoteResponse{Data: &output}
}

// List lists notes through the shared application service.
func (a *Notes) List(ctx context.Context, input requests.ListNotesInput) ListNotesResponse {
	startedAt := time.Now()
	service, logger, err := a.dependencies(ctx)
	if err != nil {
		return ListNotesResponse{Problem: desktopProblem(ctx, logger, "notes.list", startedAt, err)}
	}

	output, err := service.List(ctx, input)
	if err != nil {
		return ListNotesResponse{Problem: desktopProblem(ctx, logger, "notes.list", startedAt, err)}
	}

	return ListNotesResponse{Data: &output}
}

func (a *Notes) dependencies(ctx context.Context) (*services.Notes, *slog.Logger, error) {
	if err := a.readiness.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("wait for notes runtime: %w", err)
	}

	return a.resolve()
}

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
