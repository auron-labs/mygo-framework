package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewProjectCommandCreatesOneSupportedTemplate(t *testing.T) {
	parent := t.TempDir()
	t.Chdir(parent)

	output, err := execute(context.Background(), New("test", NewProjectCommand()), "new", "AcmeNotes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, "created acme-notes (") {
		t.Fatalf("output = %q", output)
	}
	if _, err := os.Stat(filepath.Join(parent, "acme-notes", "go.mod")); err != nil {
		t.Fatal(err)
	}

	_, err = execute(context.Background(), New("test", NewProjectCommand()), "new", "AcmeNotes")
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite") {
		t.Fatalf("duplicate error = %v", err)
	}
}
