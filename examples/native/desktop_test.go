package main

import (
	"archive/zip"
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/jobs"
)

func TestMenuStartsImportOfRememberedFolder(t *testing.T) {
	folder := t.TempDir()
	s := newScreen(t, preferences{ImportFolder: folder})
	started := make(chan string, 1)
	s.work = func(f string) jobs.Func {
		started <- f
		return func(context.Context, jobs.Report) error { return nil }
	}
	item := s.menu().ItemByID("start-import")
	if item == nil {
		t.Fatal("the menu has no start-import item")
	}
	item.Click(item, nil)
	if got := <-started; got != folder {
		t.Fatalf("imported %q, want %q", got, folder)
	}
}

func TestOpenURLSelectsAFolderWithoutImporting(t *testing.T) {
	folder := t.TempDir()
	file := filepath.Join(folder, "note.txt")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := newScreen(t, preferences{})
	s.work = func(string) jobs.Func {
		t.Error("a link started an import")
		return nil
	}
	for _, link := range []string{
		"app://import?folder=" + url.QueryEscape(file),
		"app://import?folder=" + url.QueryEscape(filepath.Join(folder, "missing")),
		"app://other?folder=" + url.QueryEscape(folder),
		"app://import",
		"%",
	} {
		s.openURL(link)
		if got := s.prefs.Get().ImportFolder; got != "" {
			t.Fatalf("%q selected %q", link, got)
		}
	}
	s.openURL("app://import?folder=" + url.QueryEscape(folder))
	if got := s.prefs.Get().ImportFolder; got != folder {
		t.Fatalf("selected %q, want %q", got, folder)
	}
}

func TestSaveSupportBundleCopyRemovesTheTemporaryFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		saved string
		err   error
	}{
		{name: "saved", saved: "/somewhere/support-bundle.zip"},
		{name: "canceled"},
		{name: "failed", err: errors.New("copy failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var path, name string
			err := saveSupportBundleCopy(t.TempDir(), t.TempDir(), func(p, n string) (string, error) {
				path, name = p, n
				readableSupportBundle(t, p)
				return tc.saved, tc.err
			})
			if !errors.Is(err, tc.err) {
				t.Fatalf("save returned %v, want %v", err, tc.err)
			}
			if name != supportBundleName {
				t.Fatalf("offered %q, want %q", name, supportBundleName)
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary file remains after %s: %v", tc.name, err)
			}
		})
	}
}

// readableSupportBundle fails unless path is a zip archive with entries.
func readableSupportBundle(t *testing.T, path string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open support bundle: %v", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close support bundle: %v", err)
		}
	}()
	if len(reader.File) == 0 {
		t.Fatal("support bundle is empty")
	}
}
