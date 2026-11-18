package sidebar

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

func TestShellLeftHeader(t *testing.T) {
	clicks := 0
	s := newShell(ShellOptions{LeftWidth: 252, LeftHeader: func(c *ui.Context) {
		IconButton(c, lucide.ArrowLeft(), "Back").OnClick(func() { clicks++ })
		IconButton(c, lucide.ArrowRight(), "Forward")
	}, ContentTitle: func(c *ui.Context) { ui.Text(c, "Document") }})
	tt := ui.NewTester(s.view, 980, 820)
	tt.SetTitleBar(ui.TitleBar{Height: 48, Left: 80})
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	toggle, _ := tt.Find("Toggle sidebar")
	back, ok := tt.Find("Back")
	if !ok || back.X < toggle.X+toggle.W || back.Y != toggle.Y {
		t.Fatalf("back not beside toggle: %+v %+v", toggle, back)
	}
	forward, ok := tt.Find("Forward")
	if !ok || forward.X < back.X+back.W || forward.X+forward.W != 244 || forward.Y != toggle.Y {
		t.Fatalf("forward not trailing in sidebar header: %+v", forward)
	}
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatal("header drag strip swallowed action")
	}
	title, _ := tt.Find("Document")
	if title.X != 268 {
		t.Fatalf("actions moved content title: %+v", title)
	}
	if err := tt.Click("Toggle sidebar"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if _, ok := tt.Find("Back"); ok {
		t.Fatal("collapsed sidebar retained header actions")
	}
	s.SetLeftExpanded(true)
	tt.Frame()
	s.SetPage(func(*ui.Context) {})
	tt.Frame()
	if _, ok := tt.Find("Back"); ok {
		t.Fatal("replacement page retained header actions")
	}
	s.SetPage(nil)
	tt.Frame()
	if _, ok := tt.Find("Back"); !ok {
		t.Fatal("workspace did not restore header actions")
	}
}
