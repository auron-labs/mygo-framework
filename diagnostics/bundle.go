// Package diagnostics writes the support bundle a user can send with a bug
// report: versions, migration status and recent logs in one zip file.
package diagnostics

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/database"
)

// maxLogBytes bounds how much of one log file is read into the bundle.
const maxLogBytes = 20 << 20

// Options names what a bundle describes.
type Options struct {
	// Version is the application version.
	Version string
	// LogDirectory holds the rotating logs written by package logging.
	LogDirectory string
	// DatabasePath and Migrations describe the schema state; both are optional.
	DatabasePath string
	Migrations   fs.FS
}

// Write writes the bundle as a zip archive to w. Parts that cannot be read are
// recorded in the bundle instead of failing it, since the bundle exists to
// diagnose a broken installation. Logs are already free of secrets by
// package logging; the home directory is replaced with ~ here as well, because
// it carries the user's name.
func Write(ctx context.Context, w io.Writer, options Options) (err error) {
	archive := zip.NewWriter(w)
	defer func() { err = errors.Join(err, archive.Close()) }()
	redact := homeRedactor()

	info := fmt.Sprintf("version: %s\nos: %s/%s\ngo: %s\ncreated: %s\n",
		options.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), time.Now().UTC().Format(time.RFC3339))
	if err := addFile(archive, "info.txt", strings.NewReader(info)); err != nil {
		return err
	}
	if options.DatabasePath != "" && options.Migrations != nil {
		if err := addFile(archive, "migrations.txt", strings.NewReader(redact(migrationReport(ctx, options)))); err != nil {
			return err
		}
	}
	return addLogs(archive, options.LogDirectory, redact)
}

func migrationReport(ctx context.Context, options Options) string {
	statuses, exists, err := database.InspectMigrations(ctx, options.DatabasePath, options.Migrations)
	if err != nil {
		return fmt.Sprintf("inspect migrations: %v\n", err)
	}
	var report strings.Builder
	fmt.Fprintf(&report, "database: %s\nexists: %t\n", options.DatabasePath, exists)
	for _, status := range statuses {
		state := "pending"
		if status.Applied {
			state = "applied"
		}
		fmt.Fprintf(&report, "%05d  %-7s  %s\n", status.Version, state, filepath.Base(status.Path))
	}
	return report.String()
}

func addLogs(archive *zip.Writer, directory string, redact func(string) string) error {
	if directory == "" {
		return nil
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return addFile(archive, "logs/error.txt", strings.NewReader(redact(fmt.Sprintf("list logs: %v\n", err))))
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		text, err := readLog(filepath.Join(directory, name))
		if err != nil {
			text = fmt.Sprintf("read log: %v\n", err)
		}
		if err := addFile(archive, "logs/"+strings.TrimSuffix(name, ".gz"), strings.NewReader(redact(text))); err != nil {
			return err
		}
	}
	return nil
}

// readLog returns a log file's text, unpacking the compressed backups that
// rotation leaves.
func readLog(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 -- entries of the application's own log directory.
	if err != nil {
		return "", fmt.Errorf("open %q: %w", filepath.Base(path), err)
	}
	defer func() { _ = file.Close() }()
	var reader io.Reader = file
	if strings.HasSuffix(path, ".gz") {
		unzipped, err := gzip.NewReader(file)
		if err != nil {
			return "", fmt.Errorf("unpack %q: %w", filepath.Base(path), err)
		}
		defer func() { _ = unzipped.Close() }()
		reader = unzipped
	}
	text, err := io.ReadAll(io.LimitReader(reader, maxLogBytes))
	if err != nil {
		return "", fmt.Errorf("read %q: %w", filepath.Base(path), err)
	}
	return string(text), nil
}

func addFile(archive *zip.Writer, name string, content io.Reader) error {
	entry, err := archive.Create(name)
	if err != nil {
		return fmt.Errorf("add %s to the bundle: %w", name, err)
	}
	if _, err := io.Copy(entry, content); err != nil {
		return fmt.Errorf("write %s to the bundle: %w", name, err)
	}
	return nil
}

func homeRedactor() func(string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return func(text string) string { return text }
	}
	return func(text string) string { return strings.ReplaceAll(text, home, "~") }
}
