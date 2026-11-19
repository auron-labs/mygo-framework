package components

import (
	"maps"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// Key components follow the desktop's text scale, which is how phones report
// iOS Dynamic Type and Android's font scale: type grows inside them instead
// of clipping against a fixed-size container, and the installed theme keeps
// following the system's dark-mode switch at runtime.
func TestKeyComponentsGrowWithTextScale(t *testing.T) {
	date := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	selection := 0
	open := new(bool)
	heights := map[string]float32{}
	dark := false
	build := func(c *ui.Context) {
		theme.Apply(c)
		dark = c.Theme().Dark
		ui.Column(c).Padding(8).Gap(8).Children(func() {
			// The menu button leads the column: the content below grows with
			// the text scale and would otherwise push its popover off-window.
			more := IconButton(c, lucide.Ellipsis(), "More", IconButtonPlain)
			MenuButton(c, more, func(m *Menu) { m.Item("Duplicate").Chosen() })
			button := Button(c, "Save the document", ButtonPrimary)
			badge := Badge(c, "Active")
			tabs := Tabs(c, &selection, TabsUnderline, "Overview", "Details")
			item := SidebarItem(c, "Projects", nil, false)
			tree := TreeItem(c, "Folder", open, false, nil)
			cal := Calendar(c, &date)
			heights["button"] = button.LayoutBounds().H
			heights["badge"] = badge.LayoutBounds().H
			heights["tabs"] = tabs.List.LayoutBounds().H
			heights["sidebarItem"] = item.LayoutBounds().H
			heights["treeItem"] = tree.LayoutBounds().H
			heights["calendar"] = cal.LayoutBounds().H
		})
	}
	tt := ui.NewTester(build, 500, 500)
	tt.Frame()
	// Menu rows build only while the menu shows; Find reads their laid-out
	// geometry, unlike the handles above, which read the previous frame's.
	if err := tt.Click("More"); err != nil {
		t.Fatalf("opening the menu: %v", err)
	}
	row, ok := tt.Find("Duplicate")
	if !ok {
		t.Fatal("menu row not found")
	}
	base := maps.Clone(heights)
	base["menuRow"] = row.H

	tt.SetPreferences(ui.Preferences{TextScale: 2})
	tt.Frame()
	tt.Advance(300 * time.Millisecond) // let the badge's label swap finish resizing
	tt.Frame()
	scaled := maps.Clone(heights)
	if row, ok = tt.Find("Duplicate"); !ok {
		t.Fatal("menu row not found after scaling")
	}
	scaled["menuRow"] = row.H
	for name, h := range scaled {
		if h <= base[name] {
			t.Errorf("%s height = %.1f at TextScale 2, want more than %.1f at the usual scale",
				name, h, base[name])
		}
	}

	tt.SetDark(true)
	tt.Frame()
	tt.Frame()
	if !dark {
		t.Error("the theme did not follow the system's dark mode")
	}
}
