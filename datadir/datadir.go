// Package datadir resolves the directory an application stores its data in and
// removes the paths that identify the user from text that leaves the machine.
package datadir

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
)

// Default returns the directory the application stores its data in. On desktop
// it is name under the user configuration directory, so existing installations
// keep their data. On phones it is MyGo's per-app directory, which App.Name()
// chooses, and name does not apply.
func Default(name string) (string, error) {
	return defaultPath(runtime.GOOS, name,
		func() (string, error) { return mygo.App.Path(mygo.PathUserData) },
		os.UserConfigDir)
}

// defaultPath takes the platform and its lookups as arguments so tests can
// exercise the phone branch without a phone.
func defaultPath(goos, name string, userData, configDir func() (string, error)) (string, error) {
	if isPhone(goos) {
		directory, err := userData()
		if err != nil {
			return "", fmt.Errorf("resolve application data directory: %w", err)
		}
		return directory, nil
	}
	directory, err := configDir()
	if err != nil {
		return "", fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return filepath.Join(directory, name), nil
}

func isPhone(goos string) bool { return goos == "ios" || goos == "android" }

// Logs returns the directory the application writes its logs to, where each
// operating system keeps them: ~/Library/Logs/name on macOS, name\Logs under
// the local (non-roaming) application data on Windows, and the XDG state
// directory on Linux and other Unix systems. On phones it is the logs folder of
// the application's data directory.
func Logs(name string) (string, error) {
	if isPhone(runtime.GOOS) {
		directory, err := Default(name)
		if err != nil {
			return "", err
		}
		return filepath.Join(directory, "logs"), nil
	}
	return desktopLogs(runtime.GOOS, name, os.Getenv, os.UserHomeDir, os.UserCacheDir)
}

// desktopLogs takes the platform and its lookups as arguments so tests can
// exercise every platform from one machine. localData is the local application
// data directory on Windows, which Go reports as the user cache directory.
func desktopLogs(goos, name string, getenv func(string) string, home, localData func() (string, error)) (string, error) {
	switch goos {
	case "windows":
		directory, err := localData()
		if err != nil {
			return "", fmt.Errorf("resolve local application data directory: %w", err)
		}
		return filepath.Join(directory, name, "Logs"), nil
	case "darwin":
		directory, err := home()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(directory, "Library", "Logs", name), nil
	default:
		// The XDG specification ignores a relative XDG_STATE_HOME. path.IsAbs
		// applies Unix rules wherever this runs; filepath.IsAbs would reject
		// "/var/state" on Windows.
		if state := getenv("XDG_STATE_HOME"); path.IsAbs(state) {
			return filepath.Join(state, name), nil
		}
		directory, err := home()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(directory, ".local", "state", name), nil
	}
}
