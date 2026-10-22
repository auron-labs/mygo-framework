package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mygo-framework/logging"
	"mygo-framework/problem"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTool[Input, Output any](
	server *sdk.Server,
	tool *sdk.Tool,
	logger *slog.Logger,
	handler sdk.ToolHandlerFor[Input, Output],
) {
	outputSchema, err := jsonschema.For[Output](nil)
	if err != nil {
		panic(fmt.Sprintf("MCP tool %q output schema: %v", tool.Name, err))
	}
	tool.OutputSchema = outputSchema

	sdk.AddTool[Input, any](server, tool, func(
		ctx context.Context,
		request *sdk.CallToolRequest,
		input Input,
	) (*sdk.CallToolResult, any, error) {
		startedAt := time.Now()
		result, output, err := handler(ctx, request, input)
		if err == nil {
			return result, output, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, err
		}

		mapped := problem.FromError(err)
		if mapped.Code == problem.CodeInternal && logger != nil {
			logging.ReportUnexpected(ctx, logger, tool.Name, "mcp", err,
				slog.Duration("duration", time.Since(startedAt)),
			)
		}
		result = &sdk.CallToolResult{
			Content:           []sdk.Content{&sdk.TextContent{Text: mapped.Message}},
			StructuredContent: mapped,
		}
		result.SetError(err)
		return result, nil, nil
	})
}
