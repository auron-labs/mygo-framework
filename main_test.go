package main

import (
	"bytes"
	appMCP "mygo-framework/app/mcp"
	"strings"
	"testing"
)

func TestMCPOptionsFromEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  map[string]string
		enabled bool
		port    int
		wantErr string
	}{
		{name: "unset"},
		{name: "disabled", values: map[string]string{mcpEnabledEnvironment: "false", mcpPortEnvironment: "invalid"}},
		{name: "enabled", values: map[string]string{mcpEnabledEnvironment: "true", mcpPortEnvironment: "48123"}, enabled: true, port: 48123},
		{name: "invalid enabled", values: map[string]string{mcpEnabledEnvironment: "sometimes"}, wantErr: mcpEnabledEnvironment},
		{name: "missing port", values: map[string]string{mcpEnabledEnvironment: "true"}, wantErr: mcpPortEnvironment + " is required"},
		{name: "zero port", values: map[string]string{mcpEnabledEnvironment: "true", mcpPortEnvironment: "0"}, wantErr: "1 to 65535"},
		{name: "invalid port", values: map[string]string{mcpEnabledEnvironment: "true", mcpPortEnvironment: "70000"}, wantErr: "1 to 65535"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options, err := mcpOptionsFromEnvironment(func(name string) (string, bool) {
				value, exists := test.values[name]
				return value, exists
			})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil || options.Enabled != test.enabled || options.Port != test.port {
				t.Fatalf("options = %+v, error = %v", options, err)
			}
		})
	}
}

func TestMCPDisclosureRequiresExplicitEnabledAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		exists  bool
		enabled bool
		want    bool
		wantErr string
	}{
		{name: "unset"},
		{name: "false", value: "false", exists: true, enabled: true},
		{name: "enabled", value: "true", exists: true, enabled: true, want: true},
		{name: "without MCP", value: "true", exists: true, wantErr: mcpEnabledEnvironment},
		{name: "invalid", value: "sometimes", exists: true, enabled: true, wantErr: mcpDiscloseEnvironment},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			disclose, err := mcpDisclosureFromEnvironment(func(name string) (string, bool) {
				if name != mcpDiscloseEnvironment {
					return "", false
				}
				return test.value, test.exists
			}, test.enabled)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil || disclose != test.want {
				t.Fatalf("disclose = %t, error = %v", disclose, err)
			}
		})
	}
}

func TestWriteMCPConnectionDetails(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeMCPConnectionDetails(&output, appMCP.ConnectionDetails{
		Endpoint:    "http://127.0.0.1:48123/mcp",
		BearerToken: "launch-token",
	})
	if err != nil {
		t.Fatalf("write connection details: %v", err)
	}
	if got := output.String(); got != "MCP endpoint: http://127.0.0.1:48123/mcp\nMCP authorization: Bearer launch-token\n" {
		t.Fatalf("connection output = %q", got)
	}
}
