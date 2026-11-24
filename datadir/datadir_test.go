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

func TestDesktopLogsUseEachPlatformsLogLocation(t *testing.T) {
	home := func() (string, error) { return "/home/user", nil }
	localData := func() (string, error) { return "/home/user/AppData/Local", nil }
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	tests := []struct {
		name, goos string
		env        map[string]string
		want       string
	}{
		{"macOS", "darwin", nil, filepath.Join("/home/user", "Library", "Logs", "Notes")},
		{"Windows", "windows", nil, filepath.Join("/home/user/AppData/Local", "Notes", "Logs")},
		{"Linux default", "linux", nil, filepath.Join("/home/user", ".local", "state", "Notes")},
		{"Linux XDG_STATE_HOME", "linux", map[string]string{"XDG_STATE_HOME": "/var/state"}, filepath.Join("/var/state", "Notes")},
		{"Linux relative XDG_STATE_HOME", "linux", map[string]string{"XDG_STATE_HOME": "state"}, filepath.Join("/home/user", ".local", "state", "Notes")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := desktopLogs(test.goos, "Notes", env(test.env), home, localData)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("desktopLogs = %q, want %q", got, test.want)
			}
		})
	}
}
