package components

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTimePicker(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := time.Date(2024, 2, 29, 12, 30, 45, 123, time.FixedZone("Test", 3600))
		original := value
		changes := 0
		disabled := false
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Disabled(disabled).Children(func() {
				picker := TimePicker(c, &value)
				picker.Label("Time")
				if picker.Changed() {
					changes++
				}
			})
		}, 400, 400)
		tt.SetDark(dark)
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		isOpen := func() bool { _, ok := tt.Find("Hours"); return ok }
		click("Time")
		if !isOpen() {
			t.Fatal("did not open")
		}
		hours, _ := tt.Find("Hours")
		minutes, _ := tt.Find("Minutes")
		if hours.X+hours.W > minutes.X || hours.Y != minutes.Y || hours.H != minutes.H {
			t.Fatalf("hour/minute columns not aligned: %+v %+v", hours, minutes)
		}
		// Hiding the tracks must not disable independent wheel scrolling.
		beforeHour, _ := tt.Find("Hours 12")
		beforeMinute, _ := tt.Find("Minutes 30")
		tt.Scroll(beforeMinute.X+beforeMinute.W/2, beforeMinute.Y+beforeMinute.H/2, 0, 32)
		tt.Frame()
		afterHour, _ := tt.Find("Hours 12")
		afterMinute, _ := tt.Find("Minutes 30")
		if beforeMinute.Y == afterMinute.Y || beforeHour.Y != afterHour.Y {
			t.Fatal("minute wheel scrolling did not remain independent")
		}
		tt.Key(0, ui.KeyDown)
		if value != original || changes != 0 {
			t.Fatal("hour draft changed value")
		}
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyDown)
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if !isOpen() || changes != 1 || value.Hour() != 13 || value.Minute() != 31 {
			t.Fatalf("keyboard selection: %s, changes %d, open %v", value, changes, isOpen())
		}
		if value.Day() != 29 || value.Month() != time.February || value.Year() != 2024 || value.Second() != 45 || value.Nanosecond() != 123 || value.Location() != original.Location() {
			t.Fatal("selection lost date or precision")
		}
		click("Time") // Close explicitly before reopening.
		click("Time")
		// Opening reveals the selected values in each independently scrolled column.
		click("Hours 13")
		click("Minutes 31")
		if !isOpen() || changes != 1 {
			t.Fatal("same-value confirmation")
		}
		click("Time")
		click("Time")
		tt.Key(0, ui.KeyHome)
		tt.Key(0, ui.KeyEscape)
		click("Time")
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if changes != 1 {
			t.Fatal("cancelled draft survived reopening")
		}
		click("Time")
		click("Time")
		tt.Key(0, ui.KeyEnd)
		tt.Key(0, ui.KeyRight)
		tt.Key(0, ui.KeyEnd)
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if value.Hour() != 23 || value.Minute() != 59 || changes != 2 {
			t.Fatal("cannot reach last hour/minute", value)
		}
		click("Time")
		click("Time")
		tt.Key(0, ui.KeyHome)
		tt.ClickAt(390, 390)
		if isOpen() || value.Hour() != 23 {
			t.Fatal("outside press did not cancel draft")
		}
		click("Time")
		value = time.Date(2025, 1, 1, 2, 3, 0, 0, time.UTC)
		tt.Frame()
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if value.Hour() != 2 || value.Minute() != 3 || changes != 2 {
			t.Fatal("external update lost")
		}
		disabled = true
		tt.Frame()
		click("Time")
		if isOpen() {
			t.Fatal("disabled picker opened")
		}
	}
}

func TestTimePickerPointerCommit(t *testing.T) {
	value := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		picker := TimePicker(c, &value)
		picker.Label("Time")
		if picker.Changed() {
			changes++
		}
	}, 400, 400)
	for _, label := range []string{"Time", "Hours 02", "Minutes 05"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	if value.Hour() != 2 || value.Minute() != 5 || changes != 1 {
		t.Fatalf("pointer selection: %s changes %d", value, changes)
	}
	if _, ok := tt.Find("Hours"); !ok {
		t.Fatal("pointer selection dismissed the popup")
	}
	// Continue adjusting minutes without reopening or losing column focus.
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if value.Minute() != 6 || changes != 2 {
		t.Fatalf("continued selection: %s changes %d", value, changes)
	}
	if _, ok := tt.Find("Hours"); !ok {
		t.Fatal("continued selection dismissed the popup")
	}
}
