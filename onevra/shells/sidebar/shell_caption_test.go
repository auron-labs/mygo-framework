package sidebar

import (
	"runtime"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

type captionWindow struct {
	fakeWindow
	maximized         bool
	minimized, closed int
}

func (w *captionWindow) Minimize()         { w.minimized++ }
func (w *captionWindow) ToggleMaximize()   { w.maximized = !w.maximized }
func (w *captionWindow) IsMaximized() bool { return w.maximized }
func (w *captionWindow) Close()            { w.closed++ }

func TestCaptionActions(t *testing.T) {
	s := newShell(ShellOptions{})
	w := &captionWindow{}
	s.host = w
	tt := ui.NewTester(s.captionButtons, 400, 48)
	for _, label := range []string{"Minimize window", "Maximize window", "Restore window", "Close window"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if w.minimized != 1 || w.closed != 1 || w.maximized {
		t.Fatalf("unexpected caption actions: %+v", w)
	}
}

func TestCaptionGlyphSize(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, dark := range []bool{false, true} {
			s := newShell(ShellOptions{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.Root().Background(ui.Transparent)
				ui.Column(c).Children(func() {
					s.captionButtons(c)
					ui.Row(c).Children(func() {
						IconButton(c, lucide.X(), "Toolbar close")
						IconButton(c, lucide.Square(), "Toolbar maximize")
						iconButton(c, lucide.Minus(), "Reference minus", c.Theme().Rem(0.875))
					})
				})
			}, 400, 200)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{TextScale: scale})
			tt.Move(399, 199)
			caption, ok := tt.Find("Close window")
			toolbar, toolbarOK := tt.Find("Toolbar close")
			if !ok || !toolbarOK || caption.W != toolbar.W || caption.H != toolbar.H || caption.W < 32 {
				t.Fatalf("caption hit target changed: %+v vs %+v", caption, toolbar)
			}
			img := tt.Image()
			inkWidth := func(r ui.Rect) int {
				left, right := int(r.X+r.W), int(r.X)-1
				for y := int(r.Y); y < int(r.Y+r.H); y++ {
					for x := int(r.X); x < int(r.X+r.W); x++ {
						if img.RGBAAt(x, y).A != 0 {
							left, right = min(left, x), max(right, x)
						}
					}
				}
				return max(0, right-left+1)
			}
			if small, regular := inkWidth(caption), inkWidth(toolbar); small == 0 || small >= regular {
				t.Fatalf("scale %v: close glyph %d should be smaller than toolbar glyph %d", scale, small, regular)
			}
			maximize, _ := tt.Find("Maximize window")
			regularMaximize, _ := tt.Find("Toolbar maximize")
			if small, regular := inkWidth(maximize), inkWidth(regularMaximize); small == 0 || small >= regular {
				t.Fatalf("scale %v: maximize glyph %d should be smaller than toolbar glyph %d", scale, small, regular)
			}
			minus, _ := tt.Find("Minimize window")
			referenceMinus, _ := tt.Find("Reference minus")
			// The vertical offset changes subpixel rasterization at fractional scales.
			if got, want := inkWidth(minus), inkWidth(referenceMinus); got == 0 || got < want-2 || got > want+2 {
				t.Fatalf("scale %v: minimize glyph width %d, want %d", scale, got, want)
			}
		}
	}
}

func TestCaptionFeedback(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 1.5, 2} {
			s := newShell(ShellOptions{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.Root().Background(ui.Transparent)
				s.captionButtons(c)
			}, 400, 100)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{TextScale: scale})
			tt.Move(399, 99)
			minus, _ := tt.Find("Minimize window")
			closeRect, _ := tt.Find("Close window")
			img := tt.Image()
			top, bottom := int(minus.Y+minus.H), -1
			for y := int(minus.Y); y < int(minus.Y+minus.H); y++ {
				for x := int(minus.X); x < int(minus.X+minus.W); x++ {
					if img.RGBAAt(x, y).A != 0 {
						top, bottom = min(top, y), max(bottom, y)
					}
				}
			}
			center := float32(top+bottom+1) / 2
			want := minus.Y + minus.H/2 + 2*scale
			if bottom < 0 || center < want-1 || center > want+1 {
				t.Fatalf("minimize glyph center %v, want %v", center, want)
			}
			x, y := closeRect.X+5, closeRect.Y+5
			if img.RGBAAt(int(x), int(y)).A != 0 {
				t.Fatal("close button should rest transparent")
			}
			for _, pressed := range []bool{false, true} {
				tt.Move(x, y)
				if pressed {
					tt.Press(x, y)
				}
				img = tt.Image()
				face := img.RGBAAt(int(x), int(y))
				red, green, blue := uint8(232), uint8(17), uint8(35)
				if pressed {
					red, green, blue = 196, 43, 28
				}
				if face.R != red || face.G != green || face.B != blue || face.A != 255 {
					t.Fatalf("unexpected close feedback: %+v", face)
				}
				white := false
				for py := int(closeRect.Y); py < int(closeRect.Y+closeRect.H); py++ {
					for px := int(closeRect.X); px < int(closeRect.X+closeRect.W); px++ {
						v := img.RGBAAt(px, py)
						white = white || (v.R > 230 && v.G > 230 && v.B > 230 && v.A == 255)
					}
				}
				if !white {
					t.Fatal("close glyph should be white on red feedback")
				}
			}
			tt.Release(399, 99)
			tt.Move(399, 99)
			if tt.Image().RGBAAt(int(x), int(y)).A != 0 {
				t.Fatal("close feedback remains after pointer leaves")
			}
		}
	}
}

func TestWindowsCaptionPages(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows caption layout")
	}
	s := newShell(ShellOptions{})
	for _, page := range []func(*ui.Context){nil, func(c *ui.Context) { ui.Text(c, "Replacement") }} {
		s.SetPage(page)
		for _, scale := range []float32{1, 1.5, 2} {
			tt := ui.NewTester(s.view, 360, 480)
			tt.SetPreferences(ui.Preferences{TextScale: scale})
			previous := float32(0)
			for _, label := range []string{"Minimize window", "Maximize window", "Close window"} {
				r, ok := tt.Find(label)
				if !ok || r.X < previous || r.X+r.W > 360 || r.Y < 0 || r.Y+r.H > max(48, 48*scale) {
					t.Fatalf("scale %v, %s: %+v", scale, label, r)
				}
				previous = r.X + r.W
			}
		}
	}
}
