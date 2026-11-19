package components

import (
	"fmt"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// CalendarControl exposes layout and frame-local date changes. Keep this handle
// when applying Element layout methods, which return ui.Element.
type CalendarControl struct {
	ui.Element
	changed bool
	chosen  bool
}

// Changed reports a selected date changed by input, not cursor navigation.
func (c *CalendarControl) Changed() bool { return c.changed }

func calendarDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func calendarMonth(d time.Time, delta int) time.Time {
	first := time.Date(d.Year(), d.Month()+time.Month(delta), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(d.Day(), last)-1)
}

func calendarSetDay(value *time.Time, day time.Time) bool {
	next := time.Date(day.Year(), day.Month(), day.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), value.Location())
	if next.Equal(*value) {
		return false
	}
	*value = next
	return true
}

type calendarView uint8

const (
	calendarDays calendarView = iota
	calendarMonths
	calendarYears
)

type calendarState struct {
	cursor, last time.Time
	view         calendarView
}

func calendarYearStart(year int) int { return year - ((year%12 + 12) % 12) }
func (s *calendarState) page(delta int) {
	months := 1
	if s.view == calendarMonths {
		months = 12
	}
	if s.view == calendarYears {
		months = 144
	}
	s.cursor = calendarMonth(s.cursor, delta*months)
}

// Calendar builds the six-week view (Monday-first by default) used inside
// CalendarPicker. Arrows move its cursor; Page Up/Down change months with day
// clamping; Home/End move to month boundaries. Enter/Space or a day click selects
// a date, preserving its time and location. Navigation alone does not select.
// Click the title to switch from days to months, then to a twelve-year grid.
// Choose a year to open months, and a month to open days. In these grids,
// arrows move by cell and Page Up/Down changes pages. Today returns to today's
// day view without committing a value. All modes retain the same size, which
// grows with the desktop's text scale and otherwise keeps MyGo's spacing.
// Key or disable a parent before construction.
func Calendar(c *ui.Context, value *time.Time) *CalendarControl {
	words := localized(c)
	if value == nil {
		panic("components.Calendar: nil date")
	}
	t := c.Theme()
	// The cell takes the height of a day's type, so a larger text scale grows
	// the grid rather than clipping the numbers inside their cells. AlignSelf
	// keeps a parent column from stretching the grid wider than its cells.
	// A coarse pointer takes TouchMin's square, which a phone window still
	// fits: 7 cells and the gaps come to about 311 of its 390 DIPs.
	_, line := c.MeasureText(0, ui.Span{Text: "88"})
	touch := TouchMin(c)
	_, titleLine := c.MeasureText(0, ui.Span{Text: "August 2006", Weight: 700})
	cellW, cellH := max(t.Space(8), line, touch), max(t.Space(7), line+2, touch)
	headerH, weekdayH := max(t.Space(8), titleLine+2, touch), max(t.Space(4), line)
	gap, sectionMargin := t.Space(.5), t.Space(2)
	// Reserve the day grid's full height in all three views.
	height := max(t.Space(62), headerH+weekdayH+6*cellH+7*gap+2*sectionMargin)
	out := &CalendarControl{Element: ui.Column(c).MinWidth(cellW * 7).Height(height).AlignSelf(ui.Start).Shrink(0).Gap(gap).Focusable().Role(ui.RoleTable)}
	st := ui.Local(out.Element, "calendar", func() calendarState { return calendarState{cursor: calendarDay(*value), last: *value} })
	if st.last != *value {
		st.cursor, st.last, st.view = calendarDay(*value), *value, calendarDays
	}
	choose := func(d time.Time) {
		out.chosen = true
		out.changed = calendarSetDay(value, d)
		st.cursor, st.last = d, *value
	}
	if !out.IsDisabled() && st.view != calendarDays {
		delta := 0
		switch {
		case out.Shortcut(0, ui.KeyLeft):
			delta = -1
		case out.Shortcut(0, ui.KeyRight):
			delta = 1
		case out.Shortcut(0, ui.KeyUp):
			delta = -3
		case out.Shortcut(0, ui.KeyDown):
			delta = 3
		case out.Shortcut(0, ui.KeyPageUp):
			st.page(-1)
		case out.Shortcut(0, ui.KeyPageDown):
			st.page(1)
		case out.Shortcut(0, ui.KeyEnter), out.Shortcut(0, ui.KeySpace):
			st.view--
		}
		if st.view == calendarYears {
			delta *= 12
		}
		if delta != 0 {
			st.cursor = calendarMonth(st.cursor, delta)
		}
	} else if !out.IsDisabled() {
		switch {
		case out.Shortcut(0, ui.KeyLeft):
			st.cursor = st.cursor.AddDate(0, 0, -1)
		case out.Shortcut(0, ui.KeyRight):
			st.cursor = st.cursor.AddDate(0, 0, 1)
		case out.Shortcut(0, ui.KeyUp):
			st.cursor = st.cursor.AddDate(0, 0, -7)
		case out.Shortcut(0, ui.KeyDown):
			st.cursor = st.cursor.AddDate(0, 0, 7)
		case out.Shortcut(0, ui.KeyPageUp):
			st.cursor = calendarMonth(st.cursor, -1)
		case out.Shortcut(0, ui.KeyPageDown):
			st.cursor = calendarMonth(st.cursor, 1)
		case out.Shortcut(0, ui.KeyHome):
			st.cursor = st.cursor.AddDate(0, 0, 1-st.cursor.Day())
		case out.Shortcut(0, ui.KeyEnd):
			st.cursor = time.Date(st.cursor.Year(), st.cursor.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		case out.Shortcut(0, ui.KeyEnter), out.Shortcut(0, ui.KeySpace):
			choose(st.cursor)
		}
	}
	out.Children(func() {
		ui.Row(c).FillWidth().Height(headerH).Margin(0, 0, sectionMargin, 0).AlignItems(ui.Center).Children(func() {
			title := words.date(st.cursor, "January 2006")
			titleLabel := words.ChooseMonth
			previous, next := words.PreviousMonth, words.NextMonth
			if st.view == calendarMonths {
				title = st.cursor.Format("2006")
				titleLabel = words.ChooseYear
				previous, next = words.PreviousYear, words.NextYear
			}
			if st.view == calendarYears {
				first := calendarYearStart(st.cursor.Year())
				title = fmt.Sprintf("%d–%d", first, first+11)
				titleLabel = words.ChooseYear
				previous, next = words.PreviousYears, words.NextYears
			}
			heading := Button(c, "", ButtonGhost).Width(0).Grow(1).MinWidth(0).MinHeight(t.Space(8)).Padding(0).
				Justify(ui.Start).Background(ui.Color{}).Label(titleLabel)
			heading.Children(func() { Text(c, title).FillWidth().MinWidth(0).SingleLine().Bold() })
			if !heading.IsDisabled() && heading.Clicked() {
				if st.view < calendarYears {
					st.view++
				}
				out.Focus()
			}
			today := Button(c, "", ButtonGhost).Size(max(t.Space(7), touch), max(t.Space(8), touch)).Padding(t.Space(1.5)).Label(words.Today)
			today.Children(func() { Icon(c, lucide.RotateCcw()) })
			if !today.IsDisabled() && today.Clicked() {
				st.cursor, st.view = calendarDay(time.Now().In(value.Location())), calendarDays
				out.Focus()
			}
			nav := func(label string, delta int, icon *ui.SVG) {
				b := Button(c, "", ButtonGhost).Size(max(t.Space(7), touch), max(t.Space(8), touch)).Padding(t.Space(1.5)).Label(label)
				b.Children(func() { Icon(c, icon) })
				if !b.IsDisabled() && b.Clicked() {
					st.page(delta)
					out.Focus()
				}
			}
			nav(previous, -1, lucide.ChevronLeft())
			nav(next, 1, lucide.ChevronRight())
		})
		if st.view != calendarDays {
			calendarPeriodGrid(c, out, st, *value)
			return
		}
		month := time.Date(st.cursor.Year(), st.cursor.Month(), 1, 0, 0, 0, 0, time.UTC)
		out.Label(words.date(month, "January 2006")).Value(words.date(st.cursor, "January 2, 2006"))
		ui.Row(c).Height(weekdayH).Margin(0, 0, sectionMargin, 0).Children(func() {
			for col := range 7 {
				label := words.Weekdays[(int(words.firstWeekday())+col)%7]
				Text(c, label).Width(cellW).SingleLine().TextAlign(ui.Center).TextColor(t.TextMuted).Role(ui.RoleColumnHeader)
			}
		})
		start := month.AddDate(0, 0, -((int(month.Weekday()) - int(words.firstWeekday()) + 7) % 7))
		today := calendarDay(time.Now().In(value.Location()))
		for week := range 6 {
			ui.Row(c).MinHeight(cellH).Role(ui.RoleRow).Children(func() {
				for col := range 7 {
					day := start.AddDate(0, 0, week*7+col)
					selected := day.Equal(calendarDay(*value))
					variant := ButtonGhost
					if selected {
						variant = ButtonPrimary
					}
					cell := Button(c, fmt.Sprint(day.Day()), variant).Width(cellW).MinHeight(cellH).Padding(0).Label(words.date(day, "January 2, 2006")).Checked(selected)
					if !selected {
						if day.Month() != month.Month() {
							cell.TextColor(t.TextMuted)
						}
						if day.Equal(st.cursor) {
							cell.Background(t.SurfaceHover)
						}
						if day.Equal(today) {
							cell.Border(1, t.Accent)
						}
					}
					if day.Equal(st.cursor) {
						out.ActiveDescendant(cell)
					}
					if !cell.IsDisabled() && cell.Clicked() {
						choose(day)
					}
				}
			})
		}
	})
	dimmed(out.Element)
	return out
}

func calendarPeriodGrid(c *ui.Context, out *CalendarControl, st *calendarState, value time.Time) {
	words := localized(c)
	t := c.Theme()
	view := st.view
	year := st.cursor.Year()
	start := calendarYearStart(year)
	out.Label(words.MonthSelection).Value(words.date(st.cursor, "January 2006"))
	if view == calendarYears {
		out.Label(words.YearSelection).Value(fmt.Sprint(year))
	}
	// Keep identities distinct when switching between month and year grids.
	ui.Column(c).Key(fmt.Sprint(view)).FillWidth().Grow(1).Gap(t.Space(1)).Children(func() {
		for row := range 4 {
			ui.Row(c).FillWidth().Grow(1).Gap(t.Space(1)).Role(ui.RoleRow).Children(func() {
				for col := range 3 {
					index := row*3 + col
					target := calendarMonth(st.cursor, index+1-int(st.cursor.Month()))
					label, text := words.date(target, "January 2006"), words.date(target, "Jan")
					selected := value.Year() == year && int(value.Month()) == index+1
					atCursor := int(st.cursor.Month()) == index+1
					if view == calendarYears {
						target = calendarMonth(st.cursor, (start+index-year)*12)
						label = fmt.Sprint(start + index)
						text = label
						selected = value.Year() == start+index
						atCursor = year == start+index
					}
					variant := ButtonGhost
					if selected {
						variant = ButtonPrimary
					}
					cell := Button(c, text, variant).Width(0).Grow(1).FillHeight().Padding(0).Label(label).Checked(selected)
					if atCursor {
						out.ActiveDescendant(cell)
						if !selected {
							cell.Background(t.SurfaceHover)
						}
					}
					if !cell.IsDisabled() && cell.Clicked() {
						st.cursor = target
						st.view = view - 1
						out.Focus()
					}
				}
			})
		}
	})
}

// CalendarPicker displays a leading calendar icon and an ISO date. Click,
// Enter or Space opens Calendar in a dismissible Popover; choosing a date closes
// it and restores trigger focus, including when choosing the existing date.
// Escape/outside press cancel navigation. The application owns value; selection
// preserves its time of day and location. Changed excludes opening/navigation.
// Key or disable a parent before construction; Label names the trigger.
func CalendarPicker(c *ui.Context, value *time.Time) *CalendarControl {
	words := localized(c)
	if value == nil {
		panic("components.CalendarPicker: nil date")
	}
	t := c.Theme()
	out := &CalendarControl{Element: pickerTrigger(c, lucide.CalendarDays())}
	open := ui.Local(out.Element, "calendar-open", func() bool { return false })
	if out.IsDisabled() {
		*open = false
	} else if out.Clicked() {
		*open = !*open
	}
	Popover(c, out.Element, open, func(panel ui.Element) {
		panel.Padding(t.Space(3)).Gap(0)
		view := Calendar(c, value)
		view.AutoFocus()
		out.changed = view.changed
		if view.chosen {
			*open = false
			out.Focus()
		}
	})
	out.Children(func() {
		Text(c, words.date(*value, "2006-01-02")).SingleLine().FontFeatures("tnum")
	})
	out.Expanded(*open).Value(words.date(*value, "2006-01-02"))
	return out
}
