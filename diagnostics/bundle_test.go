package diagnostics

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/auron-labs/mygo-framework/database"
)

func readBundle(t *testing.T, data []byte) map[string]string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(content)
	}
	return files
}

func TestBundleHoldsVersionMigrationsAndRedactedLogs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory to redact")
	}
	directory := t.TempDir()
	migrations := fstest.MapFS{"00001_notes.sql": {Data: []byte("-- +goose Up\nCREATE TABLE notes (id INTEGER);\n")}}
	databasePath := filepath.Join(directory, "app.db")
	db, err := database.Open(t.Context(), databasePath, migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	logs := filepath.Join(directory, "logs")
	if err := os.MkdirAll(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logs, "app.log"), []byte("open "+filepath.Join(home, "notes.db")+" failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var packed bytes.Buffer
	zipper := gzip.NewWriter(&packed)
	_, _ = zipper.Write([]byte("older line\n"))
	_ = zipper.Close()
	if err := os.WriteFile(filepath.Join(logs, "app-2026-01-01.log.gz"), packed.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	var bundle bytes.Buffer
	err = Write(t.Context(), &bundle, Options{Version: "1.4.2", LogDirectory: logs, DatabasePath: databasePath, Migrations: migrations})
	if err != nil {
		t.Fatal(err)
	}

	files := readBundle(t, bundle.Bytes())
	if !strings.Contains(files["info.txt"], "version: 1.4.2") {
		t.Errorf("info = %q", files["info.txt"])
	}
	if !strings.Contains(files["migrations.txt"], "00001  applied  00001_notes.sql") {
		t.Errorf("migrations = %q", files["migrations.txt"])
	}
	if got := files["logs/app.log"]; !strings.Contains(got, "open ~") || strings.Contains(got, home) {
		t.Errorf("log = %q, want the home directory replaced", got)
	}
	if files["logs/app-2026-01-01.log"] != "older line\n" {
		t.Errorf("rotated log = %q", files["logs/app-2026-01-01.log"])
	}
}

func TestBundleRecordsWhatItCannotRead(t *testing.T) {
	var bundle bytes.Buffer
	err := Write(t.Context(), &bundle, Options{
		Version:      "1",
		LogDirectory: filepath.Join(t.TempDir(), "missing"),
		DatabasePath: filepath.Join(t.TempDir(), "app.db"),
		Migrations:   fstest.MapFS{"bad.txt": {}},
	})
	if err != nil {
		t.Fatal(err)
	}
	files := readBundle(t, bundle.Bytes())
	if _, hasLogs := files["logs/app.log"]; hasLogs || !strings.Contains(files["migrations.txt"], "inspect migrations") {
		t.Fatalf("files = %v", files)
	}
}
