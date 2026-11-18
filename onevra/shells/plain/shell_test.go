package plain

import (
	"runtime"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type testWindow struct {
	minimized, closed, frames int
	maximized                 bool
	title                     string
}

func (w *testWindow) Invalidate()       { w.frames++ }
func (w *testWindow) Minimize()         { w.minimized++ }
func (w *testWindow) Close()            { w.closed++ }
func (w *testWindow) ToggleMaximize()   { w.maximized = !w.maximized }
func (w *testWindow) IsMaximized() bool { return w.maximized }
func (w *testWindow) Title() string     { return w.title }

func TestHeaderShowsWindowTitle(t *testing.T) {
	s := newShell(ShellOptions{Title: "Draft"})
	tt := ui.NewTester(s.view, 760, 560)
	if !tt.HasText("Draft") {
		t.Fatalf("header missing construction title: %v", tt.Texts())
	}
	w := &testWindow{title: "Saved"}
	s.host = w
	tt.Frame()
	if !tt.HasText("Saved") {
		t.Fatalf("header did not follow the window title: %v", tt.Texts())
	}
}

func TestNativeConfiguration(t *testing.T) {
	s := newShell(ShellOptions{Hidden: true, Width: 200, Height: 100})
	o := s.nativeOptions()
	if !o.Hidden || o.TitleBarStyle != mygo.TitleBarHidden || o.Frameless || o.HideWindowButtons != (runtime.GOOS == "windows") {
		t.Fatalf("unexpected native chrome: %+v", o)
	}
	if o.Width < o.MinWidth || o.Height < o.MinHeight {
		t.Fatal("initial window violates minimum")
	}
	if o.TrafficLightPosition == nil || *o.TrafficLightPosition != (mygo.Point{X: 16, Y: 16}) {
		t.Fatal("missing traffic light placement")
	}
	if o.StateKey != "" {
		t.Fatalf("unrequested state key: %q", o.StateKey)
	}
	if o := newShell(ShellOptions{StateKey: "settings"}).nativeOptions(); o.StateKey != "settings" {
		t.Fatalf("state key: %q", o.StateKey)
	}
}

func TestContentAndDialogClearance(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		open := false
		var height float32
		s := newShell(ShellOptions{Content: func(c *ui.Context) {
			height = HeaderHeight(c)
			components.Button(c, "Body action", components.ButtonPrimary)
			components.Dialog(c, &open, components.DialogOptions{Label: "Settings"}, func(p ui.Element) { p.Width(1000).Height(1000) })
		}})
		tt := ui.NewTester(s.view, 760, 560)
		tt.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
		r, ok := tt.Find("Body action")
		if !ok || r.Y < height {
			t.Fatalf("content overlaps header: %+v / %v", r, height)
		}
		open = true
		tt.Move(0, 0)
		r, ok = tt.Find("Settings")
		if !ok || r.Y < height {
			t.Fatalf("dialog overlaps header: %+v / %v", r, height)
		}
		tt.ClickAt(120, height/2)
		if !open {
			t.Fatal("dragging strip dismissed modal")
		}
	}
}

func TestContentBehindHeader(t *testing.T) {
	s := newShell(ShellOptions{ContentBehindHeader: true, Content: func(c *ui.Context) {
		components.Button(c, "Body action", components.ButtonPrimary)
	}})
	tt := ui.NewTester(s.view, 760, 560)
	if r, ok := tt.Find("Body action"); !ok || r.Y != 0 {
		t.Fatalf("content not behind header: %+v", r)
	}
}

func TestCaptionActionsAndTheme(t *testing.T) {
	s := newShell(ShellOptions{})
	w := &testWindow{}
	s.host = w
	tt := ui.NewTester(s.captionButtons, 400, 60)
	for _, name := range []string{"Minimize window", "Maximize window", "Restore window", "Close window"} {
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	if w.minimized != 1 || w.closed != 1 || w.maximized {
		t.Fatalf("caption actions: %+v", w)
	}
	tt.SetLocale("zh-Hans")
	if err := tt.Click("关闭窗口"); err != nil {
		t.Fatal(err)
	}
	if w.closed != 2 {
		t.Fatalf("localized caption: %+v", w)
	}
	var dark bool
	s.options.Theme = func(*ui.Context) *ui.Theme { return theme.DarkPalette() }
	s.options.Content = func(c *ui.Context) { dark = c.Theme().Dark }
	ui.NewTester(s.view, 760, 560)
	if !dark {
		t.Fatal("theme not installed before content")
	}
}

func TestHeaderActionsSitInTheHeader(t *testing.T) {
	clicked := false
	var height float32
	s := newShell(ShellOptions{Title: "Draft", Content: func(c *ui.Context) { height = HeaderHeight(c) }, HeaderActions: func(c *ui.Context) {
		components.Button(c, "Settings", components.ButtonToolbar).OnClick(func() { clicked = true })
	}})
	tt := ui.NewTester(s.view, 760, 560)
	r, ok := tt.Find("Settings")
	if !ok || r.Y+r.H > height || r.X < 400 {
		t.Fatalf("header action misplaced: %+v / %v", r, height)
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("header action did not click")
	}
}

func TestHeaderLeadingReplacesTitle(t *testing.T) {
	var height float32
	s := newShell(ShellOptions{Title: "Draft", Content: func(c *ui.Context) { height = HeaderHeight(c) }, HeaderLeading: func(c *ui.Context) {
		components.Button(c, "Note", components.ButtonToolbar)
	}})
	tt := ui.NewTester(s.view, 760, 560)
	r, ok := tt.Find("Note")
	if !ok || r.Y+r.H > height || r.X > 100 {
		t.Fatalf("leading content misplaced: %+v / %v", r, height)
	}
	if tt.HasText("Draft") {
		t.Fatal("leading content did not replace the header title")
	}
}

func TestShellBuildsLocalizedToasts(t *testing.T) {
	closed := 0
	s := newShell(ShellOptions{Content: func(c *ui.Context) {
		if components.Button(c, "Save", components.ButtonPrimary).Clicked() {
			c.AddToast(ui.Toast{Title: "Saved", Timeout: -1, OnClose: func() { closed++ }})
		}
	}})
	tt := ui.NewTester(s.view, 760, 560)
	tt.SetLocale("zh-Hans")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("toast close button closed %d toasts", closed)
	}
}
