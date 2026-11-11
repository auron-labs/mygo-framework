package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/auron-labs/mygo-framework/jobs"
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
