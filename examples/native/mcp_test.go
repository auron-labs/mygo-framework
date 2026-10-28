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
	"github.com/egoist/mygo/ui"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNativeMCP(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	fixture := nativeFixture()
	ready := make(chan struct{})
	factory := fixture.Factory
	fixture.Factory = func() (func(*ui.Context), func(), error) {
		defer close(ready)
		return factory()
	}
	finished := make(chan error, 1)
	go func() { finished <- mygomcp.Run(ctx, address, []mygomcp.Fixture{fixture, nativeFixture()}) }()
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
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("MCP fixture did not start")
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "native-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint: "http://" + address + "/mcp", DisableStandaloneSSE: true, MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	var discovered struct {
		Sessions []struct {
			ID   string `json:"session_id"`
			Name string `json:"name"`
		} `json:"sessions"`
	}
	callNativeTool(ctx, t, session, "mygo_sessions", nil, &discovered)
	if len(discovered.Sessions) != 2 || discovered.Sessions[0].Name != appName {
		t.Fatalf("unexpected fixtures: %+v", discovered)
	}
	id := discovered.Sessions[0].ID
	var observation struct {
		ID      string   `json:"observation_id"`
		Labels  []string `json:"labels"`
		Targets []struct {
			Label  string `json:"label"`
			Bounds struct {
				Y float32 `json:"y"`
			} `json:"bounds"`
		} `json:"targets"`
	}
	checkboxY := func() float32 {
		for _, target := range observation.Targets {
			if target.Label == "Toggle compact spacing" {
				return target.Bounds.Y
			}
		}
		t.Fatal("checkbox target missing")
		return 0
	}
	callNativeTool(ctx, t, session, "mygo_inspect", map[string]any{"session_id": id}, &observation)
	if !slices.Contains(observation.Labels, "Compact spacing") || observation.ID == "" {
		t.Fatalf("native view not inspectable: %+v", observation)
	}
	beforeY := checkboxY()
	result := callNativeTool(ctx, t, session, "mygo_click_label", map[string]any{
		"session_id": id, "observation_id": observation.ID, "label": "Toggle compact spacing", "screenshot": true,
	}, &observation)
	if checkboxY() >= beforeY {
		t.Fatal("MCP click did not apply compact spacing")
	}
	callNativeTool(ctx, t, session, "mygo_inspect", map[string]any{"session_id": discovered.Sessions[1].ID}, &observation)
	if checkboxY() != beforeY {
		t.Fatal("fixture state leaked into another fixture")
	}
	for _, content := range result.Content {
		if image, ok := content.(*sdk.ImageContent); ok {
			decoded, err := png.Decode(bytes.NewReader(image.Data))
			if err != nil {
				t.Fatal(err)
			}
			if image.MIMEType != "image/png" || decoded.Bounds().Dx() != 640 || decoded.Bounds().Dy() != 360 {
				t.Fatalf("unexpected screenshot: %s %v", image.MIMEType, decoded.Bounds())
			}
			return
		}
	}
	t.Fatal("MCP click did not return a screenshot")
}

func callNativeTool(ctx context.Context, t *testing.T, session *sdk.ClientSession, name string, args any, out any) *sdk.CallToolResult {
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
