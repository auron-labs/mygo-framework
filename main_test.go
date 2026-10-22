package main

import (
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
