package generator

import (
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const maxMigrationVersion = 99999

var (
	scaffoldNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(?:[-_][A-Za-z0-9]+)*$`)
	migrationPattern    = regexp.MustCompile(`^([0-9]{5})_(.+)\.sql$`)
)

// ScaffoldConfig selects one focused application scaffold.
type ScaffoldConfig struct {
	Dir  string
	Kind string
	Name string
}

// ScaffoldResult identifies the new project-relative file.
type ScaffoldResult struct {
	Path string
}

type scaffoldName struct {
	Go    string
	Snake string
}

// Scaffold creates one focused file without replacing existing work.
func Scaffold(config ScaffoldConfig) (ScaffoldResult, error) {
	directory := config.Dir
	if directory == "" {
		directory = "."
	}
	name, err := parseScaffoldName(config.Name)
	if err != nil {
		return ScaffoldResult{}, err
	}

	path, content, err := scaffoldContent(directory, config.Kind, name)
	if err != nil {
		return ScaffoldResult{}, err
	}
	fullPath := filepath.Join(directory, filepath.FromSlash(path))
	if err := writeNewFile(fullPath, content); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ScaffoldResult{}, fmt.Errorf("refuse to overwrite existing file %s", path)
		}
		return ScaffoldResult{}, fmt.Errorf("write scaffold %s: %w", path, err)
	}
	return ScaffoldResult{Path: path}, nil
}

func parseScaffoldName(value string) (scaffoldName, error) {
	if value == "" || value != strings.TrimSpace(value) || !scaffoldNamePattern.MatchString(value) {
		return scaffoldName{}, fmt.Errorf("invalid scaffold name %q: use letters and digits separated by '-' or '_'", value)
	}

	var snake strings.Builder
	for index := range len(value) {
		character := value[index]
		if character == '-' || character == '_' {
			snake.WriteByte('_')
			continue
		}
		if character >= 'A' && character <= 'Z' {
			previousIsLower := index > 0 && isLowerOrDigit(value[index-1])
			nextIsLower := index+1 < len(value) && value[index+1] >= 'a' && value[index+1] <= 'z'
			previousIsUpper := index > 0 && value[index-1] >= 'A' && value[index-1] <= 'Z'
			if snake.Len() > 0 && (previousIsLower || previousIsUpper && nextIsLower) {
				snake.WriteByte('_')
			}
			character += 'a' - 'A'
		}
		snake.WriteByte(character)
	}

	parts := strings.Split(snake.String(), "_")
	var goName strings.Builder
	for _, part := range parts {
		goName.WriteString(strings.ToUpper(part[:1]))
		goName.WriteString(part[1:])
	}
	if isWindowsReservedName(snake.String()) {
		return scaffoldName{}, fmt.Errorf("invalid scaffold name %q: filename is reserved on Windows", value)
	}
	return scaffoldName{Go: goName.String(), Snake: snake.String()}, nil
}

func isLowerOrDigit(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

func isWindowsReservedName(name string) bool {
	upper := strings.ToUpper(name)
	if upper == "CON" || upper == "PRN" || upper == "AUX" || upper == "NUL" {
		return true
	}
	return len(upper) == 4 && (strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT")) && upper[3] >= '1' && upper[3] <= '9'
}

func scaffoldContent(directory, kind string, name scaffoldName) (string, []byte, error) {
	var path, source string
	switch kind {
	case "service":
		path = filepath.ToSlash(filepath.Join("app", "services", name.Snake+".go"))
		source = fmt.Sprintf(serviceScaffold, name.Go, name.Go, name.Go, name.Go, name.Go, name.Go, name.Go)
	case "request":
		path = filepath.ToSlash(filepath.Join("app", "requests", name.Snake+".go"))
		source = fmt.Sprintf(requestScaffold, name.Go, name.Go)
	case "model":
		path = filepath.ToSlash(filepath.Join("app", "models", name.Snake+".go"))
		source = fmt.Sprintf(modelScaffold, name.Go, name.Go)
	case "command":
		path = filepath.ToSlash(filepath.Join("app", "commands", name.Snake+".go"))
		source = fmt.Sprintf(commandScaffold, name.Go, name.Go, name.Snake)
	case "migration":
		migrationPath, err := nextMigrationPath(directory, name.Snake)
		if err != nil {
			return "", nil, err
		}
		return migrationPath, []byte(migrationScaffold), nil
	default:
		return "", nil, fmt.Errorf("unsupported scaffold kind %q", kind)
	}

	formatted, err := format.Source([]byte(source))
	if err != nil {
		return "", nil, fmt.Errorf("format %s scaffold: %w", kind, err)
	}
	return path, formatted, nil
}

func nextMigrationPath(directory, name string) (string, error) {
	migrationDirectory := filepath.Join(directory, "database", "migrations")
	entries, err := os.ReadDir(migrationDirectory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read migrations: %w", err)
	}

	maxVersion := 0
	for _, entry := range entries {
		match := migrationPattern.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil {
			continue
		}
		if match[2] == name {
			return filepath.ToSlash(filepath.Join("database", "migrations", entry.Name())), nil
		}
		version, parseErr := strconv.Atoi(match[1])
		if parseErr != nil {
			return "", fmt.Errorf("parse migration version %s: %w", match[1], parseErr)
		}
		maxVersion = max(maxVersion, version)
	}
	if maxVersion >= maxMigrationVersion {
		return "", errors.New("migration version limit reached")
	}
	filename := fmt.Sprintf("%05d_%s.sql", maxVersion+1, name)
	return filepath.ToSlash(filepath.Join("database", "migrations", filename)), nil
}

func writeNewFile(path string, content []byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create scaffold directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644) // #nosec G304,G302 -- validated names stay under fixed project directories and source files are conventionally readable.
	if err != nil {
		return fmt.Errorf("create scaffold file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close scaffold: %w", closeErr)
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err = file.Write(content); err != nil {
		return fmt.Errorf("write scaffold content: %w", err)
	}
	return nil
}

const serviceScaffold = `package services

// %s implements the %s feature.
type %s struct{}

// New%s constructs the service from its direct dependencies.
func New%s() *%s {
	return &%s{}
}
`

const requestScaffold = `package requests

// %sInput is validated at the service boundary.
type %sInput struct{}
`

const modelScaffold = `package models

// %s is persisted application data.
type %s struct {
	ID uint ` + "`gorm:\"primaryKey\" json:\"id\"`" + `
}
`

const commandScaffold = `package commands

import (
	"context"

	"github.com/spf13/cobra"
)

// New%sCommand constructs the application command with its initialized dependencies.
func New%sCommand(run func(context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use:  %q,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return run(command.Context())
		},
	}
}
`

const migrationScaffold = `-- +goose Up
-- Write the forward migration here.

-- +goose Down
-- Write the rollback migration here.
`
