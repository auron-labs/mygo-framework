// Package profile resolves the application's shared on-disk paths.
package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Name is the stable application profile name shared with MyGo.
	Name             = "MyGo Framework Notes"
	databaseFilename = "notes.db"
)

// Paths contains the filesystem locations needed by desktop and offline commands.
type Paths struct {
	DataDirectory string
	LogDirectory  string
	DatabasePath  string
}

// Resolve returns the application paths. A non-empty dataDirectory overrides
// the platform profile and keeps its logs alongside it.
func Resolve(dataDirectory string) (Paths, error) {
	logDirectory := ""
	if dataDirectory == "" {
		configDirectory, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve user configuration directory: %w", err)
		}
		dataDirectory = filepath.Join(configDirectory, Name)

		if runtime.GOOS == "darwin" {
			homeDirectory, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, fmt.Errorf("resolve user home directory: %w", err)
			}
			logDirectory = filepath.Join(homeDirectory, "Library", "Logs", Name)
		}
	}

	absoluteDataDirectory, err := filepath.Abs(dataDirectory)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve data directory %q: %w", dataDirectory, err)
	}
	if logDirectory == "" {
		logDirectory = filepath.Join(absoluteDataDirectory, "logs")
	}

	return Paths{
		DataDirectory: absoluteDataDirectory,
		LogDirectory:  logDirectory,
		DatabasePath:  filepath.Join(absoluteDataDirectory, databaseFilename),
	}, nil
}
