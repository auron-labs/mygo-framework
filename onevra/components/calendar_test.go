package components

import (
	"bytes"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestCalendarPickerTextBoxAppearance(t *testing.T) {
	for _, mode := range []bool{false, true} {
		date := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
		value := date.Format("2006-01-02")
		picker := true
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			if picker {
				CalendarPicker(c, &date).Width(200).Height(32)
			} else {
				TextBox(c, &value, TextBoxOptions{LeadingIcon: lucide.CalendarDays()}).Width(200).Height(32)
			}
		}, 220, 50)
		first := append([]byte(nil), tt.Image().Pix...)
		picker = false
		tt.Frame()
		img := tt.Image()
		// Compare the leading inset, including the surface, border and muted icon.
		for y := 0; y < 32; y++ {
			for x := 0; x < 30; x++ {
				i := img.PixOffset(x, y)
				if !bytes.Equal(first[i:i+4], img.Pix[i:i+4]) {
					t.Fatalf("picker differs from TextBox at %d,%d in theme %v", x, y, mode)
				}
			}
		}
	}
}

func TestCalendarMonthClamping(t *testing.T) {
	for _, tc := range []struct {
		date, want string
		delta      int
	}{
		{"2024-01-31", "2024-02-29", 1}, {"2023-03-31", "2023-02-28", -1}, {"2024-12-31", "2025-01-31", 1},
	} {
		d, _ := time.Parse("2006-01-02", tc.date)
		if got := calendarMonth(d, tc.delta).Format("2006-01-02"); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}

func TestCalendarPickerInteraction(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := time.Date(2024, 1, 31, 12, 34, 56, 789, time.FixedZone("Test", 3600))
		original := value
		changes := 0
		disabled := false
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Disabled(disabled).Children(func() {
				picker := CalendarPicker(c, &value)
				picker.Label("Date").Width(400)
				if picker.Changed() {
					changes++
				}
			})
		}, 600, 500)
		tt.SetDark(dark)
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		click("Date")
		if _, ok := tt.Find("Next month"); !ok {
			t.Fatal("calendar closed")
		}
		tt.Key(0, ui.KeyPageDown)
		if value != original || changes != 0 {
			t.Fatal("navigation changed value")
		}
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if value.Format("2006-01-02") != "2024-02-29" || value.Hour() != 12 || value.Nanosecond() != 789 || value.Location() != original.Location() || changes != 1 {
			t.Fatalf("selection: %s changes %d", value, changes)
		}
		if _, ok := tt.Find("Next month"); ok {
			t.Fatal("selection did not close")
		}
		click("Date")
		click("February 29, 2024")
		tt.Frame()
		if changes != 1 {
			t.Fatal("same day reported change")
		}
		if _, ok := tt.Find("Next month"); ok {
			t.Fatal("same day did not close")
		}
		click("Date")
		tt.Key(0, ui.KeyRight)
		tt.Key(0, ui.KeyEscape)
		click("Date")
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if changes != 1 {
			t.Fatal("reopening retained cancelled cursor")
		}
		disabled = true
		tt.Frame()
		click("Date")
		if _, ok := tt.Find("Next month"); ok {
			t.Fatal("disabled picker opened")
		}
	}
}

func TestCalendarFixedGeometry(t *testing.T) {
	var bounds ui.Rect
	value := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	tt := ui.NewTester(func(c *ui.Context) {
		Calendar(c, &value).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
	}, 600, 500)
	tt.Frame()
	initial := bounds
	value = value.AddDate(0, 1, 0)
	tt.Frame()
	if bounds != initial || bounds.W <= 0 || bounds.H <= 0 {
		t.Fatalf("unstable geometry: %+v %+v", initial, bounds)
	}
}
