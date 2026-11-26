package main

import (
	"context"
	"mygo-framework/locales"
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/jobs"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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

func TestImportNotificationSelectsTheImportPage(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	s.showWindow(pageImport)
	if s.kept.page != pageImport {
		t.Fatalf("page = %q, want %q", s.kept.page, pageImport)
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
	if o.Tabs == nil || o.TopActions == nil || o.OnBack == nil {
		t.Fatalf("phone navigation options = %+v", o)
	}
}

// The phone's tab bar carries the navigation routes: their labels and icons,
// the current one selected, and a tap navigating to it. Footer routes stay
// off the bar.
func TestPhoneTabsFollowRoutes(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	s.kept.page = pageImport
	tabs := s.tabs(nil)
	if len(tabs) != 2 {
		t.Fatalf("tabs = %d, want the two navigation routes", len(tabs))
	}
	if tabs[0].Label != locales.NavHome() || tabs[1].Label != locales.NavImport() {
		t.Fatalf("tab labels = %q, %q", tabs[0].Label, tabs[1].Label)
	}
	if tabs[0].Icon == nil || tabs[1].Icon == nil {
		t.Fatal("a tab lacks its icon")
	}
	if !tabs[1].Selected || tabs[0].Selected || tabs[0].Badge != 0 {
		t.Fatalf("selection or badge = %+v", tabs)
	}
	tabs[0].OnSelect()
	if s.kept.page != pageHome {
		t.Fatalf("a tab tap selected %q", s.kept.page)
	}
}

// phoneTester shows the phone layout in a phone-sized window with an iPhone's
// safe area, on the page with navigation id page.
func phoneTester(s *screen, page string) *ui.Tester {
	s.onPhone = true
	s.kept.page = page
	s.version = "0.1.0"
	tt := ui.NewTester(s.view, 390, 844)
	tt.SetInsets(47, 12, 34, 9)
	tt.Frame()
	return tt
}

// tap touches a control by its label, as a finger does.
func tap(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("missing %q to tap: %q", label, tt.Texts())
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.FingerDown(x, y)
	tt.FingerUp(x, y)
	tt.Frame()
}

// Back returns from a pushed footer route to the page before it, and a launch
// that restores a footer route has none and backs out to Home.
func TestPhoneBackReturnsToThePageBefore(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	s.onPhone = true
	s.navigate(pageImport)
	s.navigate(pageSettings)
	s.back()
	if s.current().id != pageImport {
		t.Fatalf("back went to %q, want %q", s.current().id, pageImport)
	}
	restored := newScreen(t, preferences{Profile: "Default"})
	restored.onPhone = true
	restored.kept.page = pageSettings
	restored.back()
	if restored.current().id != pageHome {
		t.Fatalf("back from a restored footer route went to %q", restored.current().id)
	}
}

// The system's back key and the top bar's back button each take one page off
// the router's history, and a finger drives the tabs and the top bar inside a
// phone's safe area.
func TestPhoneNavigationByFingerAndBack(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	tt := phoneTester(s, pageHome)
	if !tt.HasText(locales.HomeWelcome(appName)) {
		t.Fatalf("first page = %q", tt.Texts())
	}
	tap(t, tt, locales.NavImport())
	if s.current().id != pageImport || !tt.HasText(locales.ChooseFiles()) {
		t.Fatalf("Import tab shows %q", tt.Texts())
	}
	// Settings opens as a pushed screen: no tab bar, a back button.
	tap(t, tt, locales.NavGoTo(locales.NavSettings()))
	if s.current().id != pageSettings {
		t.Fatalf("page = %q", s.current().id)
	}
	if _, ok := tt.AccessByRole(ui.RoleTab, locales.NavImport()); ok {
		t.Fatal("the tab bar shows on a pushed screen")
	}
	// The system's back key returns to the tab before Settings, once.
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	tt.Advance(time.Second)
	if s.current().id != pageImport {
		t.Fatalf("back key went to %q, want %q", s.current().id, pageImport)
	}
	tap(t, tt, locales.NavGoTo(locales.NavSettings()))
	tap(t, tt, "Back")
	tt.Advance(time.Second)
	if s.current().id != pageImport {
		t.Fatalf("back button went to %q, want %q", s.current().id, pageImport)
	}
}

// The page MyGo keeps follows the router, so the system's back is what a later
// launch restores too.
func TestPhoneKeptPageFollowsTheRouter(t *testing.T) {
	s := newScreen(t, preferences{Profile: "Default"})
	tt := phoneTester(s, pageImport)
	tap(t, tt, locales.NavGoTo(locales.NavSettings()))
	if s.kept.page != pageSettings {
		t.Fatalf("kept page = %q", s.kept.page)
	}
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	tt.Advance(time.Second)
	if s.kept.page != pageImport {
		t.Fatalf("kept page after back = %q", s.kept.page)
	}
}

// Every control on the phone's screens has an accessible name and a role.
func TestPhoneControlsHaveNamesAndRoles(t *testing.T) {
	named := []ui.Role{
		ui.RoleButton, ui.RoleLink, ui.RoleCheckBox, ui.RoleRadio, ui.RoleSwitch, ui.RoleTab,
		ui.RoleTextField, ui.RoleComboBox, ui.RolePopUpButton, ui.RoleToggleButton, ui.RoleSlider,
	}
	s := newScreen(t, preferences{Profile: "Default"})
	s.update = &mygo.Update{Version: "2.0.0"}
	for _, page := range []string{pageHome, pageImport, pageSettings} {
		tt := phoneTester(s, page)
		controls := 0
		for _, a := range tt.AccessAll() {
			if !slices.Contains(named, a.Role) {
				continue
			}
			controls++
			if a.Name == "" {
				t.Errorf("%s: a %v at %+v has no accessible name", page, a.Role, a.Bounds)
			}
		}
		if controls == 0 {
			t.Errorf("%s: no controls found", page)
		}
		if _, ok := tt.AccessByRole(ui.RoleTab, locales.NavHome()); !ok && page != pageSettings {
			t.Errorf("%s: the tab bar has no Home tab", page)
		}
	}
}
