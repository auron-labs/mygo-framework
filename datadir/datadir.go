// Package datadir resolves the directory an application stores its data in and
// removes the paths that identify the user from text that leaves the machine.
package datadir

import (
	"fmt"
	"os"
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
