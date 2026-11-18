package components

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCalendarViewNavigation(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)
		original := value
		changes := 0
		var bounds ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			view := Calendar(c, &value)
			view.DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
			if view.Changed() {
				changes++
			}
		}, 400, 400)
		tt.SetDark(dark)
		initial := bounds
		click := func(label string) {
			t.Helper()
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if bounds != initial {
				t.Fatalf("view changed size: %+v, %+v", initial, bounds)
			}
		}
		click("Choose month")
		if !tt.HasText("Jan") || !tt.HasText("Dec") {
			t.Fatal("missing month grid")
		}
		click("Next year")
		if !tt.HasText("2025") {
			t.Fatal("month page did not advance by year")
		}
		click("Choose year")
		if !tt.HasText("2016") || !tt.HasText("2027") {
			t.Fatal("missing year grid")
		}
		click("Next years")
		if !tt.HasText("2028") || !tt.HasText("2039") {
			t.Fatal("year page did not advance by twelve years")
		}
		click("Previous years")
		click("2024")
		click("February 2024")
		if changes != 0 || value != original {
			t.Fatal("view navigation committed a date")
		}
		tt.Key(0, ui.KeyEnter)
		if value.Format("2006-01-02") != "2024-02-29" || changes != 1 {
			t.Fatalf("month selection failed to clamp day: %s", value)
		}
		click("Choose month")
		click("Choose year")
		tt.Key(0, ui.KeyRight)
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyRight)
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeyEnter)
		if value.Format("2006-01-02") != "2025-03-28" || changes != 2 {
			t.Fatalf("keyboard drill-down: %s, changes %d", value, changes)
		}
		click("Choose month")
		click("Choose year")
		click("Today")
		today := calendarDay(time.Now().In(value.Location()))
		if !tt.HasText(today.Format("January 2006")) || changes != 2 {
			t.Fatal("Today should navigate without choosing")
		}
		tt.Key(0, ui.KeyEnter)
		if !calendarDay(value).Equal(today) || value.Hour() != 12 {
			t.Fatalf("Today cursor: %s", value)
		}
	}
}

func TestCalendarPickerViewNavigation(t *testing.T) {
	value := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	tt := ui.NewTester(func(c *ui.Context) { CalendarPicker(c, &value).Label("Date") }, 400, 400)
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("Date")
	click("Choose month")
	click("Choose year")
	click("2025")
	click("March 2025")
	click("March 15, 2025")
	if value.Format("2006-01-02") != "2025-03-15" {
		t.Fatal(value)
	}
	if _, ok := tt.Find("Today"); ok {
		t.Fatal("date selection did not dismiss picker")
	}
	click("Date")
	click("Choose month")
	tt.Key(0, ui.KeyEscape)
	click("Date")
	if _, ok := tt.Find("Choose month"); !ok {
		t.Fatal("picker did not reopen in day view")
	}
}

func TestCalendarDisabledNavigation(t *testing.T) {
	value := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Disabled(true).Children(func() { Calendar(c, &value) })
	}, 400, 400)
	for _, label := range []string{"Choose month", "Today", "Next month"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		_, months := tt.Find("Month selection")
		if !tt.HasText("January 2024") || months {
			t.Fatal("disabled navigation changed view")
		}
	}
}
