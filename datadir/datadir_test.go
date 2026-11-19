package datadir

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDefaultOnAPhoneUsesTheSandboxDirectory(t *testing.T) {
	for _, goos := range []string{"ios", "android"} {
		t.Run(goos, func(t *testing.T) {
			directory, err := defaultPath(goos, "Notes",
				func() (string, error) { return "/sandbox/files", nil },
				func() (string, error) { return "", errors.New("configuration directory looked up on a phone") })
			if err != nil {
				t.Fatal(err)
			}
			if directory != "/sandbox/files" {
				t.Fatalf("Default = %q, want the sandboxed data directory", directory)
			}
		})
	}
}

func TestDefaultOnDesktopUsesTheConfigurationDirectory(t *testing.T) {
	directory, err := defaultPath("linux", "Notes",
		func() (string, error) { return "", errors.New("sandbox directory looked up on desktop") },
		func() (string, error) { return "/home/user/.config", nil })
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/home/user/.config", "Notes"); directory != want {
		t.Fatalf("Default = %q, want %q", directory, want)
	}
}
