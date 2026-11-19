package sidebar

import (
	"runtime"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/internal/phone"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// onPhone has this test's shell draw the phone layout, whatever system the
// test runs on, and restores the system afterward.
func onPhone(t *testing.T) {
	t.Helper()
	phone.GOOS = "ios"
	t.Cleanup(func() { phone.GOOS = runtime.GOOS })
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
}

// A phone's safe area: the status bar's room at the top and the home
// indicator's at the bottom, as an iPhone's.
const phoneTopInset, phoneBottomInset = 47.0, 34.0

// TestPhoneShellTabsSwitchContent draws the phone's tab bar with a phone's
// insets: the bar's labels sit above the home indicator, its background
// reaches the screen's edge, and a finger tapping a tab switches the content.
func TestPhoneShellTabsSwitchContent(t *testing.T) {
	onPhone(t)
	route := "home"
	s := newShell(ShellOptions{
		Content: func(c *ui.Context) { ui.Text(c, "Body "+route) },
		Tabs: func(*ui.Context) []Tab {
			return []Tab{
				{Icon: lucide.House(), Label: "Home", Selected: route == "home", OnSelect: func() { route = "home" }},
				{Icon: lucide.FolderInput(), Label: "Import", Badge: 2, Selected: route == "import", OnSelect: func() { route = "import" }},
			}
		},
		Theme: func(*ui.Context) *ui.Theme { return theme.LightPalette() },
	})
	s.host = &fakeWindow{}
	tt := ui.NewTester(s.view, 390, 844)
	tt.SetInsets(phoneTopInset, 12, phoneBottomInset, 9)
	if !tt.HasText("Body home") {
		t.Fatal(tt.Texts())
	}
	// The bar's labels stay above the home indicator's room.
	for _, label := range []string{"Home", "Import", "2"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("%s missing: %q", label, tt.Texts())
		}
		if r.Y+r.H > 844-phoneBottomInset {
			t.Fatalf("%s entered the home indicator's room: %+v", label, r)
		}
	}
	// The bar's background reaches the screen's edge under the indicator.
	surface := theme.LightPalette().Surface
	img := tt.Image()
	if got := img.RGBAAt(195, 840); got.R != surface.R || got.G != surface.G || got.B != surface.B {
		t.Fatalf("the bar does not paint under the home indicator: %+v want %+v", got, surface)
	}
	if img.RGBAAt(195, 780) != img.RGBAAt(195, 840) {
		t.Fatal("the bar's background breaks at the safe area")
	}
	tap(t, tt, "Import")
	tt.Frame()
	if !tt.HasText("Body import") || tt.HasText("Body home") {
		t.Fatalf("a finger tap did not switch tabs: %q", tt.Texts())
	}
}

// TestPhoneShellPushesFooterRoutes opens a footer route from the top bar: the
// tab bar goes away, a back button returns, and the Back key takes the pushed
// screen away too.
func TestPhoneShellPushesFooterRoutes(t *testing.T) {
	onPhone(t)
	route := "home"
	backs := 0
	s := newShell(ShellOptions{
		Content:      func(c *ui.Context) { ui.Text(c, "Body "+route) },
		ContentTitle: func(c *ui.Context) { components.Text(c, "Now "+route).Bold().SingleLine() },
		Tabs: func(*ui.Context) []Tab {
			return []Tab{{Label: "Home", Selected: route == "home", OnSelect: func() { route = "home" }}}
		},
		TopActions: func(c *ui.Context) {
			if IconButton(c, lucide.Settings(), "Open settings").Clicked() {
				route = "settings"
			}
		},
		OnBack: func() { route, backs = "home", backs+1 },
		Theme:  func(*ui.Context) *ui.Theme { return theme.LightPalette() },
	})
	s.host = &fakeWindow{}
	tt := ui.NewTester(s.view, 390, 844)
	tt.SetInsets(phoneTopInset, 12, phoneBottomInset, 9)
	tap(t, tt, "Open settings")
	tt.Frame()
	if !tt.HasText("Body settings") || tt.HasText("Home") {
		t.Fatalf("the pushed screen does not show the footer route: %q", tt.Texts())
	}
	if !tt.HasText("Now settings") {
		t.Fatalf("the pushed screen lacks the top bar's title: %q", tt.Texts())
	}
	if r, ok := tt.Find("Back"); !ok || r.Y < phoneTopInset {
		t.Fatalf("the back button does not sit below the status bar: %+v ok=%v", r, ok)
	}
	tap(t, tt, "Back")
	tt.Frame()
	if !tt.HasText("Body home") || !tt.HasText("Home") || backs != 1 {
		t.Fatalf("back did not return to the tabs: %q backs=%d", tt.Texts(), backs)
	}
	// The Back key, as Android's button and the keyboard's, returns too.
	tap(t, tt, "Open settings")
	tt.Frame()
	tt.Key(0, ui.KeyBack)
	tt.Frame()
	if backs != 2 || !tt.HasText("Body home") {
		t.Fatalf("the Back key did not return: %q backs=%d", tt.Texts(), backs)
	}
}

// TestPhoneShellOmitsDesktopChrome checks that the phone draws none of the
// desktop-only pieces: no sidebars, header, caption buttons, drag or resize
// handles, with content sitting inside the safe area exactly once.
func TestPhoneShellOmitsDesktopChrome(t *testing.T) {
	onPhone(t)
	s := newShell(ShellOptions{
		Left:         func(c *ui.Context) { ui.Text(c, "Navigation") },
		LeftToolbar:  func(c *ui.Context) { ui.Text(c, "Footer") },
		LeftHeader:   func(c *ui.Context) { ui.Text(c, "Left head") },
		Right:        func(c *ui.Context) { ui.Text(c, "Inspector") },
		RightHeader:  func(c *ui.Context) { ui.Text(c, "Right head") },
		ContentTitle: func(c *ui.Context) { components.Text(c, "Title").Bold() },
		Content:      func(c *ui.Context) { ui.Box(c).Fill().Label("Body") },
	})
	tt := ui.NewTester(s.view, 390, 844)
	tt.SetInsets(phoneTopInset, 12, phoneBottomInset, 9)
	r, ok := tt.Find("Body")
	if !ok {
		t.Fatal("content missing")
	}
	if r.X != 9 || r.Y != phoneTopInset || r.W != 369 || r.H != 763 {
		t.Fatalf("content not inside the safe area exactly: %+v", r)
	}
	for _, absent := range []string{
		"Navigation", "Footer", "Left head", "Inspector", "Right head",
		"Toggle sidebar", "Toggle right sidebar", "Resize sidebar", "Resize right sidebar",
		"Minimize window", "Maximize window", "Close window",
	} {
		if tt.HasText(absent) {
			t.Fatalf("phone drew the desktop chrome %q", absent)
		}
	}
	// The floating top bar clears the content the same way the desktop's
	// header does: one HeaderHeight below the safe area.
	if r, ok := tt.Find("Title"); !ok || r.Y < phoneTopInset {
		t.Fatalf("the top bar's title sits in the status bar's room: %+v ok=%v", r, ok)
	}
	// A replacement page takes the whole window: no top bar, no tabs.
	s.SetPage(func(c *ui.Context) { ui.Text(c, "Takeover") })
	tt.Frame()
	if !tt.HasText("Takeover") || tt.HasText("Title") || tt.HasText("Body") {
		t.Fatalf("a replacement page did not take the window: %q", tt.Texts())
	}
	s.SetPage(nil)
	tt.Frame()
	if !tt.HasText("Body") {
		t.Fatalf("restoring the default page lost the content: %q", tt.Texts())
	}
}

// TestPhoneToastsClearTheSafeArea checks that toasts, which draw over the bars
// in the overlay, sit inside the safe area at the edge they stack at.
func TestPhoneToastsClearTheSafeArea(t *testing.T) {
	onPhone(t)
	const w, h = 390.0, 844.0
	for _, placement := range []components.ToastPlacement{components.ToastTop, components.ToastBottom} {
		s := newShell(ShellOptions{Toasts: components.ToastOptions{Placement: placement}, Content: func(c *ui.Context) {
			if components.Button(c, "Save", components.ButtonPrimary).Clicked() {
				c.AddToast(ui.Toast{Title: "Saved", Timeout: -1})
			}
		}})
		s.host = &fakeWindow{}
		tt := ui.NewTester(s.view, 390, 844)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.SetInsets(phoneTopInset, 0, phoneBottomInset, 0)
		tap(t, tt, "Save")
		r, ok := tt.Find("Saved")
		if !ok {
			t.Fatalf("toast missing at %d", placement)
		}
		if placement.Top() {
			if r.Y < phoneTopInset || r.X < 0 || r.X+r.W > w {
				t.Fatalf("top toast not inside the safe area: %+v", r)
			}
		} else if r.Y+r.H > h-phoneBottomInset {
			t.Fatalf("bottom toast not inside the safe area: %+v", r)
		}
	}
}
