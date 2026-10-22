package profile

import (
	"path/filepath"
	"testing"
)

func TestResolveUsesExplicitDataDirectory(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "portable profile")
	paths, err := Resolve(directory)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	if paths.DataDirectory != directory {
		t.Fatalf("data directory = %q, want %q", paths.DataDirectory, directory)
	}
	if want := filepath.Join(directory, "logs"); paths.LogDirectory != want {
		t.Fatalf("log directory = %q, want %q", paths.LogDirectory, want)
	}
	if want := filepath.Join(directory, "notes.db"); paths.DatabasePath != want {
		t.Fatalf("database path = %q, want %q", paths.DatabasePath, want)
	}
}

func TestResolveUsesNamedPlatformProfile(t *testing.T) {
	paths, err := Resolve("")
	if err != nil {
		t.Fatalf("resolve default paths: %v", err)
	}
	if filepath.Base(paths.DataDirectory) != Name {
		t.Fatalf("data directory %q does not use profile name %q", paths.DataDirectory, Name)
	}
}

func TestResolveDoesNotCollapseExplicitWhitespaceToDefaultProfile(t *testing.T) {
	paths, err := Resolve(" ")
	if err != nil {
		t.Fatalf("resolve whitespace directory: %v", err)
	}
	if filepath.Base(paths.DataDirectory) != " " {
		t.Fatalf("data directory = %q, want explicit whitespace directory", paths.DataDirectory)
	}
}
