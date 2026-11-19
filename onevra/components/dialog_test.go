package components

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

func TestDialogLayers(t *testing.T) {
	for _, protected := range []bool{false, true} {
		outer, inner := true, true
		tt := ui.NewTester(func(c *ui.Context) {
			Dialog(c, &outer, DialogOptions{Label: "Outer"}, func(p ui.Element) {
				Button(c, "Outer action", ButtonPrimary)
				Dialog(c, &inner, DialogOptions{Label: "Inner", DisableBackdropClose: protected}, func(p ui.Element) {
					Button(c, "Inner action", ButtonPrimary)
				})
			})
		}, 600, 500)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.ClickAt(2, 2)
		if !outer || inner != protected {
			t.Fatalf("backdrop: outer=%v inner=%v protected=%v", outer, inner, protected)
		}
		if protected {
			tt.Key(0, ui.KeyEscape)
		}
		if inner || !outer {
			t.Fatal("top layer dismissal")
		}
		tt.Frame()
		tt.ClickAt(2, 2)
		if outer {
			t.Fatal("outer layer did not dismiss")
		}
	}
}

func TestAlertDialogResults(t *testing.T) {
	for _, action := range []string{"OK", "Cancel", "backdrop", "escape"} {
		open := true
		var results []DialogResult
		tt := ui.NewTester(func(c *ui.Context) {
			r := AlertDialog(c, &open, AlertDialogOptions{Title: "Question", Message: "Continue?", ConfirmLabel: "OK", CancelLabel: "Cancel"})
			if r != DialogNone {
				results = append(results, r)
			}
		}, 600, 500)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		switch action {
		case "backdrop":
			tt.ClickAt(2, 2)
		case "escape":
			tt.Key(0, ui.KeyEscape)
		default:
			if err := tt.Click(action); err != nil {
				t.Fatal(err)
			}
		}
		tt.Frame()
		want := DialogCancelled
		if action == "OK" {
			want = DialogConfirmed
		}
		if open || len(results) != 1 || results[0] != want {
			t.Fatalf("%s: open=%v results=%v", action, open, results)
		}
	}
}

func TestDialogViewportClearance(t *testing.T) {
	for _, size := range [][2]int{{360, 480}, {820, 600}, {820, 900}, {1440, 1000}, {600, 240}} {
		for _, height := range []float32{80, 320, 720, 1400} {
			t.Run(fmt.Sprintf("%dx%d/panel-%g", size[0], size[1], height), func(t *testing.T) {
				open := true
				var bounds ui.Rect
				tt := ui.NewTester(func(c *ui.Context) {
					Dialog(c, &open, DialogOptions{Label: "Geometry"}, func(p ui.Element) {
						p.Width(1000).Height(height).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
						Text(c, "Content")
					})
				}, size[0], size[1])
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				w, h := float32(size[0]), float32(size[1])
				if bounds.X < 19.5 || bounds.Y < 19.5 || bounds.X+bounds.W > w-19.5 || bounds.Y+bounds.H > h-19.5 {
					t.Fatalf("dialog escaped 20px margins: %+v", bounds)
				}
				wantY := 20 + (h-40-bounds.H)/3
				if math.Abs(float64(bounds.X-(w-bounds.W)/2)) > 1 || math.Abs(float64(bounds.Y-wantY)) > 1 {
					t.Fatalf("not centered with remaining space biased upward: %+v, want y=%g", bounds, wantY)
				}
				if math.Abs(float64(bounds.H-min(height, h-40))) > 1 {
					t.Fatalf("panel height unnecessarily restricted: %+v", bounds)
				}
			})
		}
	}
}

func TestDialogGeometryAndNotice(t *testing.T) {
	open := true
	var bounds ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		Dialog(c, &open, DialogOptions{DisableEscapeClose: true, DisableBackdropClose: true}, func(p ui.Element) {
			p.DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
			Text(c, "Body")
		})
	}, 280, 480)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if bounds.X < 20 || bounds.X+bounds.W > 260 || bounds.Y < 20 || bounds.Y+bounds.H > 460 || bounds.Y+bounds.H/2 >= 240 {
		t.Fatalf("bounds: %+v", bounds)
	}
	tt.Key(0, ui.KeyEscape)
	tt.ClickAt(2, 2)
	if !open {
		t.Fatal("protected dialog closed")
	}
	notice := ui.NewTester(func(c *ui.Context) {
		AlertDialog(c, &open, AlertDialogOptions{Title: "Notice", ConfirmLabel: "OK", CancelLabel: "Cancel", HideCancel: true})
	}, 400, 400)
	if notice.HasText("Cancel") || !notice.HasText("OK") {
		t.Fatal("notice actions")
	}
}

func TestDialogTransitions(t *testing.T) {
	for _, nested := range []bool{false, true} {
		outer, open := true, false
		checked, choice := false, 0
		if !nested {
			outer = false
		}
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.RGB(255, 255, 255))
			inner := func() {
				Dialog(c, &open, DialogOptions{Label: "Inner"}, func(ui.Element) {
					Text(c, "Inner")
					// Their paint callbacks must survive the exiting copy's later passes.
					IconButton(c, lucide.Settings(), "Settings", IconButtonPlain)
					Checkbox(c, &checked, "Check")
					Radio(c, &choice, 1, "Choice")
					FloatingButtonGroup(c, "Actions", FloatingButtonGroupMuted, ui.Horizontal, []FloatingButtonGroupItem{{ID: "share", Label: "Share", Toggle: true, On: true}})
				})
			}
			if nested {
				Dialog(c, &outer, DialogOptions{Label: "Outer"}, func(ui.Element) { inner() })
			} else {
				inner()
			}
		}, 400, 300)
		dim := func() uint8 { return 255 - tt.Image().RGBAAt(2, 2).R }
		tt.Frame()
		base := dim()
		open = true
		tt.Frame()
		if dim() > base {
			t.Fatalf("nested=%v: backdrop did not fade in", nested)
		}
		tt.Advance(time.Second)
		if r, ok := tt.Find("Share"); ok {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(time.Second)
		}
		if r, ok := tt.Find("Settings"); ok {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
		}
		shown := dim()
		if shown <= base {
			t.Fatalf("nested=%v: backdrop did not dim", nested)
		}
		open = false
		tt.Frame()
		if leaving := dim(); nested && leaving != base || !nested && leaving <= base {
			t.Fatalf("nested=%v: dim right after closing %d, base %d", nested, leaving, base)
		}
		tt.Advance(time.Second)
		if dim() != base || tt.HasText("Inner") {
			t.Fatalf("nested=%v: exit did not finish", nested)
		}
	}
}
