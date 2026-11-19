package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentifier(t *testing.T) {
	for name, want := range map[string]string{"arrow-up-right": "ArrowUpRight", "circle-help": "CircleHelp", "3d-box": "Icon3dBox", "x": "X"} {
		got, err := identifier(name)
		if err != nil || got != want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"", "-x", "a--b", "Foo", "a_b", "../x"} {
		if _, err := identifier(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestPinnedGeneration(t *testing.T) {
	upstream := t.TempDir()
	out := filepath.Join(t.TempDir(), "generated")
	gallery := filepath.Join(t.TempDir(), "gallery", "icons_gen.go")
	mustWrite := func(path, data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustGit := func(args ...string) string {
		t.Helper()
		result, err := git(upstream, args...)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if err := os.Mkdir(filepath.Join(upstream, "icons"), 0o750); err != nil {
		t.Fatal(err)
	}
	mustWrite(filepath.Join(upstream, "LICENSE"), "fixture license\n")
	mustWrite(filepath.Join(upstream, "icons", "search.svg"), `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>`)
	// Create out of order; output follows upstream filenames, not creation order.
	mustWrite(filepath.Join(upstream, "icons", "arrow-up.svg"), `<svg viewBox="0 0 24 24"><path d="M12 20V4"/></svg>`)
	mustGit("init")
	mustGit("add", ".")
	mustGit("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "fixture")
	commit := mustGit("rev-parse", "HEAD")
	if err := run(upstream, commit, out, gallery, false); err != nil {
		t.Fatal(err)
	}
	if err := run(upstream, commit, out, gallery, true); err != nil {
		t.Fatal(err)
	}
	catalog, err := os.ReadFile(gallery) // #nosec G304 -- gallery is inside t.TempDir.
	if err != nil || !bytes.Contains(catalog, []byte(`{"search", lucide.Search}`)) {
		t.Fatalf("missing gallery entry: %s, %v", catalog, err)
	}
	if !bytes.Contains(catalog, []byte(`{"arrow-up", lucide.ArrowUp}`)) || bytes.Index(catalog, []byte(`{"arrow-up", lucide.ArrowUp}`)) > bytes.Index(catalog, []byte(`{"search", lucide.Search}`)) {
		t.Fatal("gallery does not preserve upstream filename order")
	}
	mustWrite(gallery, "outdated")
	if err := run(upstream, commit, out, gallery, true); err == nil || !strings.Contains(err.Error(), gallery+" is not up to date") {
		t.Fatalf("gallery drift: %v", err)
	}
	unchanged, err := os.ReadFile(gallery) // #nosec G304 -- gallery is inside t.TempDir.
	if err != nil || string(unchanged) != "outdated" {
		t.Fatal("check mode wrote the catalog")
	}
	if err := run(upstream, commit, out, gallery, false); err != nil {
		t.Fatal(err)
	}
	mustWrite(filepath.Join(out, "icons_gen.go"), "outdated")
	if err := run(upstream, commit, out, gallery, true); err == nil || !strings.Contains(err.Error(), "not up to date") {
		t.Fatalf("drift: %v", err)
	}
	if err := run(upstream, commit, out, "", false); err != nil {
		t.Fatal(err)
	}
	if err := run(upstream, commit, out, "", true); err != nil {
		t.Fatal(err)
	}
	if err := run(upstream, strings.Repeat("0", 40), out, gallery, false); err == nil {
		t.Fatal("accepted wrong revision")
	}
	mustWrite(filepath.Join(upstream, "LICENSE"), "dirty")
	if err := run(upstream, commit, out, gallery, false); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("dirty checkout: %v", err)
	}
}

func TestRender(t *testing.T) {
	icons := []icon{{"search", "Search", "<svg viewBox=\"0 0 24 24\"/>\n"}}
	a, at, err := render(icons)
	if err != nil {
		t.Fatal(err)
	}
	b, bt, err := render(icons)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || !bytes.Equal(at, bt) {
		t.Fatal("nondeterministic generation")
	}
	if !bytes.Contains(a, []byte("var cacheSearch lazyIcon")) || !bytes.Contains(at, []byte("{\"search\", Search}")) {
		t.Fatal("missing accessor or test entry")
	}
}
