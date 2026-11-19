package main

import (
	"context"
	"mygo-framework/locales"
	"slices"
	"testing"

	"github.com/auron-labs/mygo-framework/jobs"

	"github.com/egoist/mygo"
)

func TestNavigationSelectsPages(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	tt := newTester(s, "")
	tt.Frame()
	if !tt.HasText(locales.HomeWelcome(appName)) || tt.HasText("Compact spacing") {
		t.Fatalf("first page = %q", tt.Texts())
	}
	for _, step := range []struct{ click, want, absent string }{
		{locales.NavGoTo("Import"), locales.DropFolder(), "Compact spacing"},
		{locales.NavGoTo("Settings"), "Compact spacing", locales.DropFolder()},
		{locales.NavGoTo("Home"), locales.HomeWelcome(appName), "Compact spacing"},
	} {
		if err := tt.Click(step.click); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !tt.HasText(step.want) || tt.HasText(step.absent) {
			t.Fatalf("after %s: %q", step.click, tt.Texts())
		}
	}
}

func TestUnknownPageFallsBackToHome(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	tt := newTester(s, "no-such-page")
	tt.Frame()
	if !tt.HasText(locales.HomeWelcome(appName)) {
		t.Fatalf("view = %q", tt.Texts())
	}
	if s.current().id != pageHome {
		t.Fatalf("current = %q", s.current().id)
	}
}

func TestRoutesAreUniqueAndHomeIsFirst(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range routes() {
		if seen[r.id] || r.view == nil || r.title == nil || r.icon == nil {
			t.Fatalf("bad route %+v", r)
		}
		seen[r.id] = true
	}
	if routes()[0].id != pageHome {
		t.Fatal("the first route is the fallback and must be Home")
	}
}

func TestImportBadgeCountsRunningImports(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default", ImportFolder: t.TempDir()})
	release := make(chan struct{})
	s.work = func(string) jobs.Func {
		return func(ctx context.Context, report jobs.Report) error {
			report(0, 1, "working")
			select {
			case <-release:
			case <-ctx.Done():
			}
			return nil
		}
	}
	tt := newTester(s, pageHome)
	tt.Frame()
	if slices.Contains(tt.Texts(), "1") {
		t.Fatalf("badge shown while idle: %q", tt.Texts())
	}
	if activeImports(s) != 0 {
		t.Fatal("no import is running")
	}
	s.beginImport()
	tt.Frame()
	if activeImports(s) != 1 || !slices.Contains(tt.Texts(), "1") {
		t.Fatalf("badge missing while importing: %q", tt.Texts())
	}
	close(release)
}

func TestFooterShowsAccountSettingsAndVersion(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Work"})
	tt := newTester(s, pageHome)
	tt.Frame()
	for _, text := range []string{"Work", locales.AccountCaption(), locales.VersionLabel("0.1.0")} {
		if !tt.HasText(text) {
			t.Errorf("footer lacks %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click(locales.NavGoTo("Settings")); err != nil {
		t.Fatal(err)
	}
	if s.kept.page != pageSettings {
		t.Fatalf("page = %q", s.kept.page)
	}
}

func TestUpdatePromptIsInTheFooterOnEveryPage(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	s.update = &mygo.Update{Version: "2.0.0"}
	for _, page := range []string{pageHome, pageImport, pageSettings} {
		tt := newTester(s, page)
		tt.Frame()
		if !tt.HasText("Version 2.0.0 is available.") || !tt.HasText("Install update") {
			t.Errorf("%s lacks the update prompt: %q", page, tt.Texts())
		}
	}
}

func TestSidebarBecomesARailBelowTheBreakpointAndKeepsTheDraggedWidth(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	tt := newTester(s, pageHome)
	tt.Frame()
	host := s.host.(*staticLeft)
	if !slices.Contains(tt.Texts(), "Home") || !slices.Contains(tt.Texts(), "Import") || !tt.HasText(locales.VersionLabel("0.1.0")) || host.width != defaultLeftWidth {
		t.Fatalf("wide view = %q, width %v", tt.Texts(), host.width)
	}
	// The user drags the separator, then the window narrows.
	host.SetLeftWidth(320)
	tt.Frame()
	tt.SetSize(railBreakpoint-100, 640)
	tt.Frame()
	if host.width != railWidth {
		t.Fatalf("rail width = %v, want %v", host.width, float32(railWidth))
	}
	if slices.Contains(tt.Texts(), "Import") || tt.HasText(locales.VersionLabel("0.1.0")) || tt.HasText(locales.NavigationLabel()) {
		t.Fatalf("rail shows labels: %q", tt.Texts())
	}
	// Navigation still works from the rail.
	if err := tt.Click(locales.NavGoTo("Import")); err != nil {
		t.Fatal(err)
	}
	if s.kept.page != pageImport {
		t.Fatalf("page = %q", s.kept.page)
	}
	// A drag in the rail does not widen it.
	host.SetLeftWidth(200)
	tt.Frame()
	if host.width != railWidth {
		t.Fatalf("dragged rail width = %v", host.width)
	}
	tt.SetSize(1000, 640)
	tt.Frame()
	if host.width != 320 || !tt.HasText(locales.VersionLabel("0.1.0")) {
		t.Fatalf("expanded width = %v, view = %q", host.width, tt.Texts())
	}
}

func TestRailOnlyResizesWhenTheWidthChanges(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	counted := &countingHost{staticLeft: staticLeft{width: defaultLeftWidth}}
	s.host = counted
	tt := newTester(s, pageHome)
	tt.SetSize(500, 640)
	for range 4 {
		tt.Frame()
	}
	if counted.calls != 1 {
		t.Fatalf("SetLeftWidth calls = %d, want 1", counted.calls)
	}
}

type countingHost struct {
	staticLeft
	calls int
}

func (h *countingHost) SetLeftWidth(w float32) { h.calls++; h.staticLeft.SetLeftWidth(w) }

func TestShellOptionsRestoreWindowStateWithSaneSizes(t *testing.T) {
	s := newScreen(t, preferences{})
	o := s.shellOptions()
	if o.StateKey == "" || !o.Hidden || o.Title != appName {
		t.Fatalf("options = %+v", o)
	}
	if o.Width < o.MinWidth || o.Height < o.MinHeight || o.MinWidth >= railBreakpoint {
		t.Fatalf("sizes = %dx%d, minimum %dx%d", o.Width, o.Height, o.MinWidth, o.MinHeight)
	}
	if o.LeftMinWidth > railWidth || o.LeftWidth < railWidth || o.Left == nil || o.Content == nil || o.ContentTitle == nil || o.LeftToolbar == nil {
		t.Fatalf("sidebar options = %+v", o)
	}
}
