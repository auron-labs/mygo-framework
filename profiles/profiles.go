// Package profiles keeps each named profile's data in its own SQLite file
// under the data directory. Profile data needs no profile column in any
// table, switching profiles is opening another file, and a profile is removed
// by deleting one file.
package profiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/auron-labs/mygo-framework/database"
)

const (
	maxNameLength = 64
	directoryName = "profiles"
)

// Windows refuses these device names as file names, with any extension.
var reservedNames = []string{
	"con", "prn", "aux", "nul",
	"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
	"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
}

// Validate reports whether name can name a profile: 1 to 64 ASCII letters,
// digits, spaces, underscores or hyphens, starting and ending with a letter or
// digit. It excludes path separators and dots so a name cannot leave the
// profiles directory.
func Validate(name string) error {
	if name == "" || len(name) > maxNameLength {
		return fmt.Errorf("profile name must be 1 to %d characters", maxNameLength)
	}
	for i := range len(name) {
		c := name[i]
		edge := i == 0 || i == len(name)-1
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case !edge && (c == ' ' || c == '_' || c == '-'):
		default:
			return errors.New("profile name may contain only letters, digits, spaces, underscores and hyphens, and must start and end with a letter or digit")
		}
	}
	if slices.Contains(reservedNames, strings.ToLower(name)) {
		return fmt.Errorf("profile name %q is reserved by the operating system", name)
	}
	return nil
}

// Path returns the database file of the named profile. Names differing only in
// case share a file, because common file systems treat them as one.
func Path(dataDir, name string) (string, error) {
	if err := Validate(name); err != nil {
		return "", err
	}
	return filepath.Join(dataDir, directoryName, strings.ToLower(name)+".db"), nil
}

// Open opens the named profile's database under dataDir, creating it and
// applying migrations.
func Open(ctx context.Context, dataDir, name string, migrations fs.FS) (*database.Database, error) {
	path, err := Path(dataDir, name)
	if err != nil {
		return nil, fmt.Errorf("open profile: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create profile directory: %w", err)
	}
	db, err := database.Open(ctx, path, migrations)
	if err != nil {
		return nil, fmt.Errorf("open profile %q: %w", name, err)
	}
	return db, nil
}
