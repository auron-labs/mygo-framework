package components

import (
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

type holdFixture struct {
	tt                 *ui.Tester
	confirmed, holding bool
	changes            int
	x, y               float32
}

func newHoldFixture(t *testing.T, options HoldToConfirmOptions) *holdFixture {
	t.Helper()
	f := &holdFixture{}
	f.tt = ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		h := HoldToConfirm(c, &f.confirmed, "Hold to delete", options)
		f.holding = h.Holding()
		if h.Changed() {
			f.changes++
		}
	}, 320, 120)
	r, ok := f.tt.Find("Hold to delete")
	if !ok {
		t.Fatal("hold button not found")
	}
	f.x, f.y = r.X+r.W/2, r.Y+r.H/2
	return f
}

func (f *holdFixture) advance(d time.Duration) {
	f.tt.Advance(d)
	f.tt.Frame()
}

func TestHoldToConfirmPointer(t *testing.T) {
	f := newHoldFixture(t, HoldToConfirmOptions{ConfirmedLabel: "Deleted", Duration: time.Second})
	f.tt.Press(f.x, f.y)
	if !f.holding {
		t.Fatal("press did not start a hold")
	}
	f.advance(600 * time.Millisecond)
	if f.confirmed || f.changes != 0 {
		t.Fatal("hold confirmed early")
	}
	f.advance(500 * time.Millisecond)
	if !f.confirmed || f.changes != 1 || f.holding {
		t.Fatalf("hold not completed: confirmed=%v changes=%d holding=%v", f.confirmed, f.changes, f.holding)
	}
	if !f.tt.HasText("Deleted") || !slices.Contains(f.tt.Announcements(), "Deleted") {
		t.Fatalf("done label not shown and announced: %v", f.tt.Announcements())
	}
	f.tt.Release(f.x, f.y)
	f.advance(2 * time.Second)
	if f.changes != 1 {
		t.Fatal("confirmed button accepted another hold")
	}

	// Resetting restores the label and accepts a new hold.
	f.confirmed = false
	f.advance(time.Second)
	if !f.tt.HasText("Hold to delete") {
		t.Fatal("reset did not restore the label")
	}
	f.tt.Press(f.x, f.y)
	f.advance(1100 * time.Millisecond)
	if f.changes != 2 {
		t.Fatalf("hold after reset: %d confirmations", f.changes)
	}
}

func TestHoldToConfirmCancels(t *testing.T) {
	f := newHoldFixture(t, HoldToConfirmOptions{Duration: time.Second})
	f.tt.Press(f.x, f.y)
	f.advance(700 * time.Millisecond)
	f.tt.Release(f.x, f.y)
	f.advance(700 * time.Millisecond)
	if f.confirmed || f.holding {
		t.Fatal("early release confirmed")
	}

	// A quick re-press resumes from the fill that remains instead of starting over.
	f.tt.Press(f.x, f.y)
	f.advance(150 * time.Millisecond)
	f.tt.Release(f.x, f.y)
	f.tt.Press(f.x, f.y)
	f.advance(900 * time.Millisecond)
	if !f.confirmed {
		t.Fatal("re-press did not resume the fill")
	}
	f.tt.Release(f.x, f.y)
	f.confirmed = false
	f.advance(time.Second)

	// Sliding well off the button cancels, as a native press does.
	f.tt.Press(f.x, f.y)
	f.advance(500 * time.Millisecond)
	f.tt.Move(f.x, f.y+200)
	f.advance(time.Second)
	f.tt.Release(f.x, f.y+200)
	if f.confirmed || f.holding {
		t.Fatal("hold continued off the button")
	}
}

func TestHoldToConfirmKeyboard(t *testing.T) {
	f := newHoldFixture(t, HoldToConfirmOptions{Duration: time.Second})
	f.tt.Key(0, ui.KeyTab)
	if !f.tt.Focused("Hold to delete") {
		t.Fatal("Tab did not focus the button")
	}
	f.tt.KeyDown(ui.KeySpace)
	f.advance(500 * time.Millisecond)
	f.tt.KeyUp(ui.KeySpace)
	f.advance(time.Second)
	if f.confirmed {
		t.Fatal("released Space confirmed")
	}
	f.tt.KeyDown(ui.KeyEnter)
	f.advance(1100 * time.Millisecond)
	f.tt.KeyUp(ui.KeyEnter)
	if !f.confirmed || f.changes != 1 {
		t.Fatalf("held Enter: confirmed=%v changes=%d", f.confirmed, f.changes)
	}
}
