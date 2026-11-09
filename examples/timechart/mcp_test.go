package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net"
	"slices"
	"testing"
	"time"

	mygomcp "github.com/auron-labs/mygo-mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestTimeChartMCP zooms the stacked charts through the MCP harness and
// checks the screenshots it returns.
func TestTimeChartMCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- mygomcp.Run(ctx, address, []mygomcp.Fixture{fixture()}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("MCP harness did not stop")
		}
	})
	client := sdk.NewClient(&sdk.Implementation{Name: "timechart-test", Version: "1"}, nil)
	var session *sdk.ClientSession
	for session == nil {
		session, err = client.Connect(ctx, &sdk.StreamableClientTransport{
			Endpoint: "http://" + address + "/mcp", DisableStandaloneSSE: true, MaxRetries: -1,
		}, nil)
		if err != nil {
			select {
			case <-ctx.Done():
				t.Fatal(err)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	var sessions struct {
		Sessions []struct {
			ID string `json:"session_id"`
		} `json:"sessions"`
	}
	callTool(ctx, t, session, "mygo_sessions", nil, &sessions)
	id := sessions.Sessions[0].ID

	// Points arrive off the UI thread, so wait until the last chart shows its events.
	var observation struct {
		ID     string   `json:"observation_id"`
		Labels []string `json:"labels"`
	}
	var result *sdk.CallToolResult
	for matched := false; !matched; {
		var waited struct {
			Matched bool `json:"matched"`
		}
		result = callTool(ctx, t, session, "mygo_wait_text", map[string]any{
			"session_id": id, "text": "Arousal", "timeout_ms": 100, "screenshot": true,
		}, &waited)
		matched = waited.Matched
		if ctx.Err() != nil {
			t.Fatal("charts never showed their data")
		}
	}
	checkScreenshot(t, result)

	callTool(ctx, t, session, "mygo_inspect", map[string]any{"session_id": id}, &observation)
	if !slices.Contains(observation.Labels, "Oxygen saturation") || !slices.Contains(observation.Labels, "23:00") {
		t.Fatalf("charts not inspectable: %+v", observation)
	}
	result = callTool(ctx, t, session, "mygo_scroll", map[string]any{
		"session_id": id, "observation_id": observation.ID, "x": 500, "y": 80, "delta_x": 0, "delta_y": -600, "screenshot": true,
	}, nil)
	checkScreenshot(t, result)
	var zoomed struct {
		Matched bool `json:"matched"`
	}
	for !zoomed.Matched && ctx.Err() == nil {
		callTool(ctx, t, session, "mygo_wait_text", map[string]any{
			"session_id": id, "text": "Apnea", "timeout_ms": 100,
		}, &zoomed)
	}
	callTool(ctx, t, session, "mygo_inspect", map[string]any{"session_id": id}, &observation)
	if slices.Contains(observation.Labels, "23:00") && slices.Contains(observation.Labels, "05:00") {
		t.Fatalf("scrolling over the first chart did not zoom the axis: %v", observation.Labels)
	}
}

func checkScreenshot(t *testing.T, result *sdk.CallToolResult) {
	t.Helper()
	for _, content := range result.Content {
		if image, ok := content.(*sdk.ImageContent); ok {
			decoded, err := png.Decode(bytes.NewReader(image.Data))
			if err != nil {
				t.Fatal(err)
			}
			if image.MIMEType != "image/png" || decoded.Bounds().Dx() != 1000 || decoded.Bounds().Dy() != 760 {
				t.Fatalf("unexpected screenshot: %s %v", image.MIMEType, decoded.Bounds())
			}
			return
		}
	}
	t.Fatal("no screenshot returned")
}

func callTool(ctx context.Context, t *testing.T, session *sdk.ClientSession, name string, args any, out any) *sdk.CallToolResult {
	t.Helper()
	result, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("%s: %v", name, result.StructuredContent)
	}
	if out != nil {
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
	return result
}
