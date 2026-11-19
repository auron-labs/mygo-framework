// Command lucidegen imports a pinned, clean Lucide checkout. Run from onevra/.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

type (
	icon       struct{ name, symbol, svg string }
	provenance struct {
		Repository string `json:"repository"`
		Commit     string `json:"commit"`
		SHA256     string `json:"inputs_sha256"`
		Count      int    `json:"icon_count"`
	}
)

func main() {
	upstream := flag.String("upstream", "", "path to a clean Lucide checkout")
	commit := flag.String("commit", "", "required full upstream commit hash")
	out := flag.String("out", "icons/lucide", "output package directory")
	gallery := flag.String("gallery-out", "", "optional gallery catalog output file")
	check := flag.Bool("check", false, "verify generated files without writing")
	flag.Parse()
	if err := run(*upstream, *commit, *out, *gallery, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func git(dir string, args ...string) (string, error) {
	b, err := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput() // #nosec G204 -- Git arguments are fixed by the generator; dir is its explicit checkout input.
	if err != nil {
		return "", fmt.Errorf("git: %w: %s", err, b)
	}
	return strings.TrimSpace(string(b)), nil
}

func run(upstream, commit, out, gallery string, check bool) error {
	if upstream == "" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) {
		return fmt.Errorf("provide -upstream and a full 40-character -commit")
	}
	head, err := git(upstream, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != commit {
		return fmt.Errorf("checkout is %s, requested %s", head, commit)
	}
	status, err := git(upstream, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("upstream checkout must be clean")
	}
	paths, err := filepath.Glob(filepath.Join(upstream, "icons", "*.svg"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no icons found")
	}
	license, err := os.ReadFile(filepath.Join(upstream, "LICENSE")) // #nosec G304 -- Read from the explicitly selected checkout.
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "LICENSE\x00%s\x00", license)
	icons := make([]icon, 0, len(paths))
	seen := map[string]string{}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".svg")
		symbol, err := identifier(name)
		if err != nil {
			return err
		}
		if previous, ok := seen[symbol]; ok {
			return fmt.Errorf("identifier collision: %s and %s", previous, name)
		}
		seen[symbol] = name
		data, err := os.ReadFile(path) // #nosec G304 -- Paths come from the checkout glob or selected generator outputs.
		if err != nil {
			return err
		}
		parsed, err := ui.ParseSVG(data)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		w, h := parsed.Size()
		if w != 24 || h != 24 {
			return fmt.Errorf("%s: unexpected size %gx%g", name, w, h)
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", name, data)
		icons = append(icons, icon{name, symbol, string(data)})
	}
	code, tests, err := render(icons)
	if err != nil {
		return err
	}
	meta, err := json.MarshalIndent(provenance{"https://github.com/lucide-icons/lucide", commit, hex.EncodeToString(hash.Sum(nil)), len(icons)}, "", "  ")
	if err != nil {
		return err
	}
	files := []struct {
		name string
		data []byte
	}{{filepath.Join(out, "icons_gen.go"), code}, {filepath.Join(out, "icons_gen_test.go"), tests}, {filepath.Join(out, "LICENSE"), license}, {filepath.Join(out, "upstream.json"), append(meta, '\n')}}
	if gallery != "" {
		catalog, err := renderGallery(icons)
		if err != nil {
			return err
		}
		files = append(files, struct {
			name string
			data []byte
		}{gallery, catalog})
	}
	old, err := os.ReadFile(filepath.Join(out, "icons_gen.go")) // #nosec G304 -- Read the selected generator output.
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	reportChanges(old, code)
	for _, file := range files {
		path := file.name
		if check {
			existing, err := os.ReadFile(path) // #nosec G304 -- Paths come from the checkout glob or selected generator outputs.
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, file.data) {
				return fmt.Errorf("%s is not up to date", path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return err
			}
			if err := os.WriteFile(path, file.data, 0o600); err != nil {
				return err
			}
		}
	}
	fmt.Printf("Validated %d icons at %s\n", len(icons), commit)
	return nil
}

func identifier(name string) (string, error) {
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(name) {
		return "", fmt.Errorf("invalid icon name %q", name)
	}
	parts := strings.Split(name, "-")
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	s := strings.Join(parts, "")
	if s[0] >= '0' && s[0] <= '9' {
		s = "Icon" + s
	}
	if !token.IsIdentifier(s) {
		return "", fmt.Errorf("invalid identifier %q", s)
	}
	return s, nil
}

func render(icons []icon) ([]byte, []byte, error) {
	var code, tests bytes.Buffer
	code.WriteString("// Code generated by lucidegen; DO NOT EDIT.\n\npackage lucide\n\nimport \"github.com/egoist/mygo/ui\"\n\n")
	tests.WriteString("// Code generated by lucidegen; DO NOT EDIT.\n\npackage lucide\n\nimport \"github.com/egoist/mygo/ui\"\n\nvar allIcons = []struct { name string; get func() *ui.SVG }{\n")
	for _, icon := range icons {
		fmt.Fprintf(&code, "// %s returns the Lucide %s icon.\n// It parses once on first use and returns the same shared SVG on subsequent calls.\nfunc %s() *ui.SVG { return cache%s.get(%q) }\n\nvar cache%s lazyIcon\n\n", icon.symbol, icon.name, icon.symbol, icon.symbol, icon.svg, icon.symbol)
		fmt.Fprintf(&tests, "{%q, %s},\n", icon.name, icon.symbol)
	}
	tests.WriteString("}\n")
	c, err := format.Source(code.Bytes())
	if err != nil {
		return nil, nil, err
	}
	t, err := format.Source(tests.Bytes())
	return c, t, err
}

// renderGallery preserves the upstream filename order used for the accessors.
// The exhaustive runtime catalog belongs to the example, not the icon package.
func renderGallery(icons []icon) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// Code generated by lucidegen; DO NOT EDIT.\n\npackage gallery\n\nimport \"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide\"\n\nvar iconCatalog = []iconEntry{\n")
	for _, icon := range icons {
		fmt.Fprintf(&b, "{%q, lucide.%s},\n", icon.name, icon.symbol)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

func reportChanges(old, current []byte) {
	pattern := regexp.MustCompile(`(?m)^func ([A-Za-z0-9]+)\(\) \*ui.SVG`)
	names := func(data []byte) map[string]bool {
		m := map[string]bool{}
		for _, match := range pattern.FindAllSubmatch(data, -1) {
			m[string(match[1])] = true
		}
		return m
	}
	before, after := names(old), names(current)
	var added, removed []string
	for name := range after {
		if !before[name] {
			added = append(added, name)
		}
	}
	for name := range before {
		if !after[name] {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	fmt.Printf("API: %d added, %d removed\n", len(added), len(removed))
	if len(removed) > 0 {
		fmt.Printf("Review removals/renames before accepting: %s\n", strings.Join(removed, ", "))
	}
}
