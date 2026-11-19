package sidebar

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShellFloatingHeader(t *testing.T) {
	for _, inset := range []bool{false, true} {
		for _, replacement := range []bool{false, true} {
			var surfaces, bodies []ui.Rect
			var height float32
			build := func(c *ui.Context) {
				height = HeaderHeight(c)
				padding := float32(0)
				if inset {
					padding = height
				}
				surface := ui.Column(c).Fill().Background(ui.RGB(30, 90, 150)).Padding(padding, 0, 0, 0)
				surface.DrawOver(func(_ *ui.Painter, r ui.Rect) { surfaces = append(surfaces, r) })
				surface.Children(func() {
					ui.Box(c).Fill().DrawOver(func(_ *ui.Painter, r ui.Rect) { bodies = append(bodies, r) })
				})
			}
			s := newShell(ShellOptions{Left: build, Content: build})
			if replacement {
				s.SetPage(build)
			}
			view := func(c *ui.Context) { surfaces = nil; bodies = nil; s.view(c) }
			tt := ui.NewTester(view, 820, 600)
			// The header grows with the desktop's text scale: 3 rem.
			for _, config := range []struct{ native, scale, want float32 }{{0, 1, 48}, {80, 1, 80}, {48, 2, ui.LightTheme().FontSize * 6}} {
				tt.SetTitleBar(ui.TitleBar{Height: config.native})
				tt.SetPreferences(ui.Preferences{TextScale: config.scale, ReduceMotion: true})
				if height != config.want {
					t.Fatalf("header height = %v, want %v", height, config.want)
				}
				for i, surface := range surfaces {
					if r := surface; r.Y != 0 || r.H != 600 {
						t.Fatalf("surface not full height: %+v", r)
					}
					want := float32(0)
					if inset {
						want = height
					}
					if r := bodies[i]; r.Y != want || r.H != 600-want {
						t.Fatalf("inset=%v: body %+v", inset, r)
					}
				}
				for _, x := range []int{180, 500} {
					p := tt.Image().RGBAAt(x, 2)
					if p.R != 30 || p.G != 90 || p.B != 150 || p.A != 255 {
						t.Fatalf("background missing behind header: %v", p)
					}
				}
			}
			if !replacement {
				if err := tt.Click("Toggle sidebar"); err != nil {
					t.Fatal(err)
				}
				if s.LeftExpanded() {
					t.Fatal("floating header did not receive click")
				}
			}
		}
	}
}
