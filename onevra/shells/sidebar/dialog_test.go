package sidebar

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

func TestShellDialogWindowDrag(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		open := true
		actions := 0
		var height float32
		content := func(c *ui.Context) {
			height = HeaderHeight(c)
			components.Dialog(c, &open, components.DialogOptions{Label: "Modal"}, func(p ui.Element) {
				p.Width(1000).Height(1000)
			})
		}
		s := newShell(ShellOptions{
			Content: content,
			ContentTitle: func(c *ui.Context) {
				if components.Button(c, "Header action", components.ButtonPrimary).Clicked() {
					actions++
				}
			},
		})
		if replacement {
			s.SetPage(content)
		}
		tt := ui.NewTester(s.view, 820, 600)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		for _, x := range []float32{2, 100, 400, 818} {
			tt.ClickAt(x, height/2)
			tt.Press(x, height/2)
			tt.Move(x+10, height+10)
			tt.Release(x+10, height+10)
			if !open || actions != 0 || !s.LeftExpanded() {
				t.Fatal("modal header leaked input")
			}
		}
		r, ok := tt.Find("Modal")
		if !ok || r.Y < height-.5 {
			t.Fatalf("panel overlaps inherited strip: %+v", r)
		}
		tt.ClickAt(2, height+10)
		if open {
			t.Fatal("backdrop no longer dismisses dialog")
		}
	}
}
