package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeCommandCreatesFocusedScaffold(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)

	output, err := execute(context.Background(), New("test", NewMakeCommand()), "make", "service", "AuditLog")
	if err != nil {
		t.Fatal(err)
	}
	if output != "created app/services/audit_log.go\n" {
		t.Fatalf("output = %q", output)
	}
	if _, err := os.Stat(filepath.Join(directory, "app", "services", "audit_log.go")); err != nil {
		t.Fatal(err)
	}

	_, err = execute(context.Background(), New("test", NewMakeCommand()), "make", "service", "AuditLog")
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestMakeCommandListsFocusedKinds(t *testing.T) {
	t.Parallel()
	help, err := execute(context.Background(), New("test", NewMakeCommand()), "make", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range scaffoldKinds {
		if !strings.Contains(help, kind+" ") {
			t.Errorf("help does not contain %q:\n%s", kind, help)
		}
	}
}
