package components

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestColorPlaneThumbOverhang(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			for _, corner := range [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				t.Run(fmt.Sprint(dark, disabled, corner), func(t *testing.T) {
					value := ui.RGB(255, 0, 0)
					off, changes := false, 0
					tt := ui.NewTester(func(c *ui.Context) {
						ui.Column(c).Padding(30).Disabled(off).Children(func() {
							if ColorView(c, &value).Changed() {
								changes++
							}
						})
					}, 360, 600)
					tt.SetDark(dark)
					r, _ := tt.Find("Saturation and brightness")
					cx, cy := r.X+r.W/2, r.Y+r.H/2
					x, y := r.X+corner[0]*r.W, r.Y+corner[1]*r.H
					tt.Press(cx, cy)
					tt.Move(x, y)
					tt.Release(x, y)
					before, count := value, changes
					off = disabled
					tt.Frame()
					// Start on the portion outside both edges of the plane.
					x += (corner[0]*2 - 1) * 4
					y += (corner[1]*2 - 1) * 4
					tt.Press(x, y)
					tt.Move(cx, cy)
					tt.Release(cx, cy)
					if disabled {
						if value != before || changes != count {
							t.Fatal("disabled plane accepted thumb input")
						}
					} else if value != ui.RGB(128, 64, 64) || changes <= count || !tt.Focused("Saturation and brightness") {
						t.Fatalf("overhang drag did not focus/update plane: %v, changes=%d", value, changes-count)
					}
				})
			}
		}
	}
}

func TestColorChannelOverhangSemantics(t *testing.T) {
	for _, hi := range []float64{1, 360} {
		for _, end := range []float64{0, hi} {
			for _, dark := range []bool{false, true} {
				value, disabled, changes := end, false, 0
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Padding(30).Disabled(disabled).Children(func() {
						e := colorChannel(c, &value, hi, "Channel", func(p *ui.Painter, r ui.Rect) {
							p.Fill(r, ui.RGB(200, 30, 100), 0)
						})
						if e.Changed() {
							changes++
						}
					})
				}, 300, 100)
				tt.SetDark(dark)
				r, _ := tt.Find("Channel")
				x, y := r.X+4, r.Y+r.H/2
				if end == hi {
					x = r.X + r.W - 4
				}
				tt.Press(x, y)
				if value != end || changes != 0 {
					t.Fatal("pressing the overhang changed an endpoint")
				}
				tt.Move(r.X+r.W/2, y)
				tt.Release(r.X+r.W/2, y)
				if value != hi/2 || changes != 1 || !tt.Focused("Channel") {
					t.Fatalf("captured drag: value=%v changes=%d", value, changes)
				}
				tt.Key(0, ui.KeyEnd)
				if value != hi || changes != 2 {
					t.Fatal("keyboard behavior lost")
				}
				value, disabled = end, true
				tt.Frame()
				tt.Press(x, y)
				tt.Move(r.X+r.W/2, y)
				tt.Release(r.X+r.W/2, y)
				if value != end || changes != 2 {
					t.Fatal("disabled channel accepted overhang input")
				}
			}
		}
	}
}
