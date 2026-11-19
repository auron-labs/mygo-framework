package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// coarse are the desktop settings a touch screen alone reports: the touch-first
// layout, with TouchMin-sized targets, follows them.
func coarse() ui.Preferences {
	return ui.Preferences{
		TextScale: 1, Pointer: ui.PointerCoarse, AnyPointer: ui.PointerCoarse,
		NoHover: true, NoAnyHover: true,
	}
}

// TestIconButtonTouchTarget checks that every icon button style takes the
// coarse pointer's minimum square; the fine pointer's compact sizes are what
// the other tests assert.
func TestIconButtonTouchTarget(t *testing.T) {
	for _, style := range []IconButtonStyle{IconButtonPlain, IconButtonToolbar, IconButtonCompact} {
		tt := ui.NewTesterWith(func(c *ui.Context) {
			IconButton(c, lucide.Settings(), "Settings", style)
		}, 200, 100, ui.TesterOptions{Preferences: coarse()})
		r, ok := tt.Find("Settings")
		if !ok {
			t.Fatalf("style %d: missing icon button", style)
		}
		if r.W < 44 || r.H < 44 {
			t.Fatalf("style %d: hit target %+v, want at least a 44×44 square", style, r)
		}
	}
}

func TestSidebarItemTouchTarget(t *testing.T) {
	tt := ui.NewTesterWith(func(c *ui.Context) {
		SidebarItem(c, "Inbox", lucide.Inbox(), false)
	}, 240, 100, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Inbox")
	if !ok || r.H < 44 {
		t.Fatalf("item %+v, want at least 44 high", r)
	}
}

func TestTagRemoveTouchTarget(t *testing.T) {
	tags := []string{"alpha", "beta"}
	changed := false
	tt := ui.NewTesterWith(func(c *ui.Context) {
		if TagInput(c, &tags).Changed() {
			changed = true
		}
	}, 400, 200, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Remove alpha")
	if !ok || r.W < 44 || r.H < 44 {
		t.Fatalf("remove target %+v, want at least a 44×44 square", r)
	}
	tt.FingerDown(r.X+r.W/2, r.Y+r.H/2)
	tt.FingerUp(r.X+r.W/2, r.Y+r.H/2)
	if !changed || len(tags) != 1 || tags[0] != "beta" {
		t.Fatalf("tap removed nothing: changed %v, tags %v", changed, tags)
	}
}

func TestMenuRowTouchTarget(t *testing.T) {
	tt := ui.NewTesterWith(func(c *ui.Context) {
		more := IconButton(c, lucide.Ellipsis(), "More", IconButtonPlain)
		MenuButton(c, more, func(m *Menu) {
			m.Item("Rename")
			m.Item("Duplicate")
		})
	}, 300, 200, ui.TesterOptions{Preferences: coarse()})
	more, _ := tt.Find("More")
	tt.FingerDown(more.X+more.W/2, more.Y+more.H/2)
	tt.FingerUp(more.X+more.W/2, more.Y+more.H/2)
	r, ok := tt.Find("Rename")
	if !ok {
		t.Fatal("the menu did not open")
	}
	if r.H < 44 {
		t.Fatalf("menu row %+v, want at least 44 high", r)
	}
}

func TestToastCloseTouchTarget(t *testing.T) {
	closed := false
	tt := ui.NewTesterWith(func(c *ui.Context) {
		c.AddToast(ui.Toast{ID: "Saved", Title: "Saved", Timeout: -1, OnClose: func() { closed = true }})
		Toasts(c)
	}, 800, 600, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Close")
	if !ok || r.W < 44 || r.H < 44 {
		t.Fatalf("close target %+v, want at least a 44×44 square", r)
	}
	tt.FingerDown(r.X+r.W/2, r.Y+r.H/2)
	tt.FingerUp(r.X+r.W/2, r.Y+r.H/2)
	if !closed {
		t.Fatal("the tap did not close the toast")
	}
}

// TestContextMenuLongPress checks that a finger held still opens the menu a
// right-click does, and that a tap on its item chooses it.
func TestContextMenuLongPress(t *testing.T) {
	chosen := ""
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() {
			row := ui.ButtonBase(c).Key("row").Label("Report").Size(200, 40)
			ContextMenu(c, row, func(m *Menu) {
				if m.Item("Open").Chosen() {
					chosen = "Open"
				}
			})
		})
	}, 500, 400, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Report")
	if !ok {
		t.Fatal("missing target")
	}
	tt.FingerDown(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(600 * time.Millisecond)
	tt.FingerUp(r.X+r.W/2, r.Y+r.H/2)
	if !tt.HasText("Open") {
		t.Fatal("a long press did not open the context menu")
	}
	item, _ := tt.Find("Open")
	tt.FingerDown(item.X+item.W/2, item.Y+item.H/2)
	tt.FingerUp(item.X+item.W/2, item.Y+item.H/2)
	if chosen != "Open" || tt.HasText("Open") {
		t.Fatalf("a tap chose %q with the menu still %v", chosen, tt.HasText("Open"))
	}
}

// TestContextMenuFitsPhoneWindow opens the menu by a long press in the bottom
// corner of a phone-sized window with safe-area insets: every item stays on
// screen, at the coarse pointer's minimum height.
func TestContextMenuFitsPhoneWindow(t *testing.T) {
	const w, h = 390.0, 844.0
	tt := ui.NewTesterWith(func(c *ui.Context) {
		card := ui.Box(c).Fill().Label("Card")
		ContextMenu(c, card, func(m *Menu) {
			for i := range 12 {
				m.Item(fmt.Sprintf("Item %02d", i))
			}
		})
	}, 390, 844, ui.TesterOptions{Preferences: coarse()})
	tt.SetInsets(47, 0, 34, 0)
	// The worst case on screen: a long press as far into the window's bottom
	// corner as the safe area leaves reachable.
	tt.FingerDown(w-6, h-40)
	tt.Advance(600 * time.Millisecond)
	tt.FingerUp(w-6, h-40)
	for i := range 12 {
		r, ok := tt.Find(fmt.Sprintf("Item %02d", i))
		if !ok {
			t.Fatalf("item %d missing", i)
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > w || r.Y+r.H > h {
			t.Fatalf("item %d spills out of the phone window: %+v", i, r)
		}
	}
	if r, _ := tt.Find("Item 00"); r.H < 44 {
		t.Fatalf("row %+v, want at least 44 high", r)
	}
}

// TestCalendarTouchTarget checks that Today, the month arrows and the day
// cells take the coarse pointer's minimum square, and that the grown grid
// still fits a phone window's width.
func TestCalendarTouchTarget(t *testing.T) {
	value := time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)
	tt := ui.NewTesterWith(func(c *ui.Context) {
		Calendar(c, &value)
	}, 400, 500, ui.TesterOptions{Preferences: coarse()})
	for _, label := range []string{"Today", "Previous month", "Next month", "November 19, 2026"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q", label)
		}
		if r.W < 44 || r.H < 44 {
			t.Fatalf("%q hit target %+v, want at least a 44×44 square", label, r)
		}
	}
	// November 2026 starts the week's first column, so its first Saturday
	// sits in the grid's last column: the whole grid ends within a phone.
	r, _ := tt.Find("November 7, 2026")
	if r.X+r.W > 390 {
		t.Fatalf("the grid spills a phone window: %+v", r)
	}
}

// TestTimePickerTouchTarget checks that the picker's rows take the coarse
// pointer's minimum square inside its scrolling columns.
func TestTimePickerTouchTarget(t *testing.T) {
	value := time.Date(2026, time.November, 19, 9, 41, 0, 0, time.UTC)
	tt := ui.NewTesterWith(func(c *ui.Context) {
		TimePicker(c, &value).Label("Time")
	}, 400, 300, ui.TesterOptions{Preferences: coarse()})
	if err := tt.Click("Time"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Hours 09", "Minutes 41"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q", label)
		}
		if r.W < 44 || r.H < 44 {
			t.Fatalf("%q hit target %+v, want at least a 44×44 square", label, r)
		}
	}
}
