package sidebar

import (
	"runtime"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

func TestShellToolbarPlacement(t *testing.T) {
	s, _ := fixture()
	var body ui.Rect
	clicks := 0
	s.options.Left = func(c *ui.Context) { ui.Box(c).Fill().DrawOver(func(_ *ui.Painter, r ui.Rect) { body = r }) }
	s.options.LeftToolbar = func(c *ui.Context) {
		ui.Spacer(c)
		if IconButton(c, lucide.Settings(), "Settings").Clicked() {
			clicks++
		}
	}
	tt := ui.NewTester(s.view, 820, 600)
	tt.SetTitleBar(ui.TitleBar{Height: 48, Left: 80})
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	left, _ := tt.Find("Toggle sidebar")
	if runtime.GOOS == "darwin" && left.X < 96 {
		t.Fatal("sidebar toggle too close to traffic lights", left)
	}
	bottom, ok := tt.Find("Settings")
	if !ok || bottom.Y+bottom.H != 592 || bottom.X+bottom.W != 212 || body.Y+body.H > bottom.Y {
		t.Fatal("toolbar not pinned below sidebar content", bottom, body)
	}
	if err := tt.Click("Settings"); err != nil || clicks != 1 {
		t.Fatal("bottom toolbar action failed", err, clicks)
	}
	s.SetLeftExpanded(false)
	tt.Frame()
	if _, ok := tt.Find("Settings"); ok {
		t.Fatal("collapsed sidebar retains toolbar")
	}
	s.SetLeftExpanded(true)
	tt.Frame()
	s.SetPage(func(*ui.Context) {})
	tt.Frame()
	if _, ok := tt.Find("Settings"); ok {
		t.Fatal("replacement page retains sidebar toolbar")
	}
	s.SetPage(nil)
	tt.Frame()
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatal("returning did not restore sidebar toolbar")
	}
}
