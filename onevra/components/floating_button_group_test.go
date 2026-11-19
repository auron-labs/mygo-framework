package components

import (
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func floatingTestItems(share string, undo bool) []FloatingButtonGroupItem {
	return []FloatingButtonGroupItem{
		{ID: "undo", Label: "Undo", Icon: lucide.Undo2, IconOnly: true, Shortcut: "⌘Z", Disabled: !undo},
		{ID: "redo", Label: "Redo", Icon: lucide.Redo2, IconOnly: true},
		{Separator: true},
		{ID: "comment", Label: "Comment", Icon: lucide.MessageCircle},
		{ID: "share", Label: share, ReserveLabels: []string{"Share", "Copied"}},
		{Separator: true},
		{ID: "present", Label: "Present", Toggle: true},
	}
}

func TestFloatingButtonGroupActions(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		orientation := ui.Horizontal
		next, back := ui.KeyRight, ui.KeyLeft
		if vertical {
			orientation, next, back = ui.Vertical, ui.KeyDown, ui.KeyUp
		}
		clicked := ""
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(false))
			ui.Column(c).Padding(40).Children(func() {
				if id := FloatingButtonGroup(c, "Board", FloatingButtonGroupMuted, orientation, floatingTestItems("Share", false)).Clicked(); id != "" {
					clicked = id
				}
			})
		}, 600, 500)
		redo, _ := tt.Find("Redo")
		comment, _ := tt.Find("Comment")
		if vertical && redo.W != comment.W {
			t.Fatalf("icon-only button %v wide, labelled %v: the rail's buttons differ", redo.W, comment.W)
		}
		if err := tt.Click("Undo"); err != nil {
			t.Fatal(err)
		}
		if clicked != "" {
			t.Fatalf("vertical=%v: disabled undo reported %q", vertical, clicked)
		}
		if err := tt.Click("Share"); err != nil {
			t.Fatal(err)
		}
		if clicked != "share" {
			t.Fatalf("vertical=%v: clicked %q, want share", vertical, clicked)
		}
		// One Tab stop; the arrows wrap and skip the disabled button.
		tt.Key(0, ui.KeyHome)
		if !tt.Focused("Redo") {
			t.Fatalf("vertical=%v: Home did not reach the first enabled button", vertical)
		}
		tt.Key(0, back)
		if !tt.Focused("Present") {
			t.Fatalf("vertical=%v: arrow did not wrap to the end", vertical)
		}
		tt.Key(0, next)
		tt.Key(0, next)
		if !tt.Focused("Comment") {
			t.Fatalf("vertical=%v: arrows did not skip the separator", vertical)
		}
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if clicked != "comment" {
			t.Fatalf("vertical=%v: Enter clicked %q", vertical, clicked)
		}
	}
}

func TestFloatingButtonGroupHighlight(t *testing.T) {
	var tray ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Column(c).Padding(40).Children(func() {
			g := FloatingButtonGroup(c, "Board", FloatingButtonGroupMuted, ui.Horizontal, floatingTestItems("Share", true))
			g.DrawOver(func(_ *ui.Painter, r ui.Rect) { tray = r })
		})
	}, 600, 200)
	tt.Frame()
	// The highlight's left edge, on a row through the buttons' lower part,
	// beneath their content, against the tray without one.
	y := int(tray.Y + tray.H - 7)
	rest := tt.Image()
	edge := func() int {
		t.Helper()
		img := tt.Image()
		for x := int(tray.X); x < int(tray.X+tray.W); x++ {
			if img.RGBAAt(x, y) != rest.RGBAAt(x, y) {
				return x
			}
		}
		return -1
	}
	redo, _ := tt.Find("Redo")
	share, _ := tt.Find("Share")
	tt.Move(redo.X+redo.W/2, redo.Y+redo.H/2)
	tt.Advance(time.Second)
	start := edge()
	if start < int(redo.X)-1 || start > int(redo.X)+3 {
		t.Fatalf("highlight at %d, want under Redo at %v", start, redo.X)
	}
	tt.Move(share.X+share.W/2, share.Y+share.H/2)
	tt.Advance(60 * time.Millisecond)
	if x := edge(); x <= start || x >= int(share.X)-12 {
		t.Fatalf("highlight jumped instead of travelling: %d (from %d to about %v)", x, start, share.X-12)
	}
	tt.Advance(time.Second)
	end := edge()
	if end <= start {
		t.Fatalf("highlight did not reach Share: %d", end)
	}
	// It stays as the pointer crosses the separator, and fades out of the tray.
	tt.Move(redo.X+redo.W+6, share.Y+share.H/2)
	tt.Advance(time.Second)
	if x := edge(); x != end {
		t.Fatalf("highlight left Share in the tray: %d", x)
	}
	tt.Move(5, 5)
	tt.Advance(time.Second)
	if x := edge(); x >= 0 {
		t.Fatal("highlight stayed after the pointer left")
	}
	// Reduced Motion lands the highlight at once.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Move(share.X+share.W/2, share.Y+share.H/2)
	tt.Frame()
	if x := edge(); x != end {
		t.Fatalf("reduced motion highlight at %d, want %d", x, end)
	}
}

func TestFloatingButtonGroupReservedLabel(t *testing.T) {
	share := "Share"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(true))
		FloatingButtonGroup(c, "Board", FloatingButtonGroupRaised, ui.Horizontal, floatingTestItems(share, true))
	}, 600, 200)
	tt.Frame()
	before, _ := tt.Find("Present")
	share = "Copied"
	tt.Frame()
	tt.Advance(time.Second)
	if after, _ := tt.Find("Present"); after != before {
		t.Fatalf("label change moved the next button: %v -> %v", before, after)
	}
	if !tt.HasText("Copied") {
		t.Fatal("label did not change")
	}
}
