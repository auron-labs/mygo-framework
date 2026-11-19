package sidebar

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShellContentTitle(t *testing.T) {
	s, _ := fixture()
	var title ui.Rect
	s.options.ContentTitle = func(c *ui.Context) {
		ui.Text(c, "Current document").SingleLine().Grow(1).MinWidth(0).DrawOver(func(_ *ui.Painter, r ui.Rect) { title = r })
	}
	tt := ui.NewTester(s.view, 820, 600)
	tt.SetTitleBar(ui.TitleBar{Height: 48, Left: 80})
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if r := title; r.X != 236 || r.Y < 0 || r.Y+r.H > 48 {
		t.Fatal("title is not on the leading main-content titlebar", r)
	}
	for _, width := range []int{360, 820} {
		for _, scale := range []float32{1, 1.5, 2} {
			for _, leftOpen := range []bool{true, false} {
				s.SetLeftExpanded(leftOpen)
				tt.SetSize(width, 600)
				tt.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
				left, _ := tt.Find("Toggle sidebar")
				r := title
				if r.X < left.X+left.W || r.X+r.W > float32(width)-8 || r.Y < 0 || r.Y+r.H > max(48, 48*scale) {
					t.Fatalf("width=%d scale=%v left=%v: title overlaps controls: %+v, left %+v", width, scale, leftOpen, r, left)
				}
			}
		}
	}
	s.SetPage(func(*ui.Context) {})
	tt.Frame()
	if tt.HasText("Current document") {
		t.Fatal("replacement page retained content title")
	}
	s.SetPage(nil)
	tt.Frame()
	if !tt.HasText("Current document") {
		t.Fatal("returning did not restore content title")
	}
}
