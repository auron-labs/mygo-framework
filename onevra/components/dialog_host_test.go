package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDialogWindowDragScope(t *testing.T) {
	height := float32(48)
	other := ui.NewTester(func(c *ui.Context) {
		if got := *dialogWindowDrag(c); got != 0 {
			t.Fatalf("another window inherited %g", got)
		}
	}, 600, 500)
	tt := ui.NewTester(func(c *ui.Context) {
		check := func(want float32) {
			t.Helper()
			if got := *dialogWindowDrag(c); got != want {
				t.Fatalf("height=%g, want %g", got, want)
			}
		}
		check(0)
		WithDialogWindowDrag(c, height, func() {
			check(height)
			other.Frame()
			WithDialogWindowDrag(c, 72, func() { check(72) })
			check(height)
			WithDialogWindowDrag(c, -1, func() { check(0) })
			check(height)
			func() {
				defer func() { _ = recover() }()
				WithDialogWindowDrag(c, 100, func() { panic("restore scope") })
			}()
			check(height)
		})
		check(0)
	}, 600, 500)
	height = 64
	tt.Frame()
}

func TestDialogInheritedWindowDrag(t *testing.T) {
	for _, tc := range []struct {
		name      string
		options   DialogOptions
		y         float32
		staysOpen bool
	}{
		{"inherit", DialogOptions{}, 24, true},
		{"below host strip", DialogOptions{}, 60, false},
		{"override", DialogOptions{WindowDragHeight: 80}, 60, true},
		{"smaller override", DialogOptions{WindowDragHeight: 16}, 24, false},
		{"disabled", DialogOptions{DisableWindowDrag: true}, 24, false},
		{"disabled explicit", DialogOptions{DisableWindowDrag: true, WindowDragHeight: 80}, 24, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outer, inner := true, true
			result := DialogNone
			tt := ui.NewTester(func(c *ui.Context) {
				WithDialogWindowDrag(c, 48, func() {
					Dialog(c, &outer, DialogOptions{}, func(ui.Element) {
						r := AlertDialog(c, &inner, AlertDialogOptions{DialogOptions: tc.options, Title: "Inner", ConfirmLabel: "OK"})
						if r != DialogNone {
							result = r
						}
					})
				})
			}, 600, 500)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.ClickAt(2, tc.y)
			if inner != tc.staysOpen || !outer {
				t.Fatalf("outer=%v inner=%v", outer, inner)
			}
			if !tc.staysOpen && result != DialogCancelled {
				t.Fatal("missing cancellation result")
			}
			if tc.staysOpen {
				tt.Key(0, ui.KeyEscape)
				if inner || !outer {
					t.Fatal("Escape did not dismiss only the inner layer")
				}
			}
			tt.Frame()
			tt.ClickAt(2, 24)
			if !outer {
				t.Fatal("outer layer did not inherit drag strip")
			}
		})
	}
}
