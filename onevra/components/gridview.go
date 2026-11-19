package components

import "github.com/egoist/mygo/ui"

// GridState keeps selection and scrolling for a GridView. Keep one state per
// grid; do not copy it after first use. Keys must be unique and comparable.
// A nil Selected and Selection disables selection. Selection holds item keys;
// Selected optionally exposes the keyboard cursor. OnChange and OnSubmit,
// rather than Element.Changed/Submitted, report selection and activation.
// Drag reordering and MyGo's private collection accessibility actions are not
// exposed by this composition.
type GridState struct {
	Selected  *int
	Selection *ui.Selection[any]
	Key       func(int) any
	Label     func(int) string
	OnChange  func()
	OnSubmit  func(int)

	scroll  ui.GridState
	choice  ListState
	cells   map[int]*ui.Handle
	handles []*ui.Handle
	columns int
}

func (s *GridState) ScrollTo(item int, align ui.Align) { s.scroll.ScrollTo(item, align) }
func (s *GridState) ScrollIntoView(item int)           { s.scroll.ScrollIntoView(item) }
func (s *GridState) Focus(c *ui.Context)               { s.scroll.Focus(c) }
func (s *GridState) Focused(c *ui.Context) bool        { return s.scroll.Focused(c) }
func (s *GridState) FocusWithin(c *ui.Context) bool    { return s.scroll.FocusWithin(c) }

// GridView builds a virtualized, responsive grid with neutral, hover-free
// selection. minWidth and height describe each cell's content area in DIPs;
// a transparent two-DIP inset on each side is reserved by MyGo. Values below
// one are clamped to one. Item spacing follows MyGo's theme-scaled grid gap.
// Give the grid a height or fill a bounded parent. Builders may run during
// layout and must not perform application work. A nil state scrolls without
// selection. MyGo owns layout, virtualization, scrolling and stable cell keys.
func GridView(c *ui.Context, s *GridState, count int, minWidth, height float32, item func(int)) ui.Element {
	count = max(0, count)
	var scroll *ui.GridState
	if s != nil {
		s.choice.Selected, s.choice.Selection = s.Selected, s.Selection
		s.choice.Key, s.choice.Label = s.Key, s.Label
		s.choice.OnChange, s.choice.OnSubmit = s.OnChange, s.OnSubmit
		s.choice.reveal = s.ScrollIntoView
		s.choice.visible = s.visible
		s.choice.sync(count)
		s.scroll.Key = s.Key
		scroll = &s.scroll
		// Read last frame's geometry before replacing the visible handles.
		// Each virtual row is built in full, including rows outside the clip.
		s.columns = s.measuredColumns(c)
		s.cells = make(map[int]*ui.Handle)
	}
	interactive := s != nil && (s.Selected != nil || s.Selection != nil)
	var grid, cursor ui.Element
	grid = ui.GridView(c, scroll, count, max(1, minWidth)+4, max(1, height)+4, func(i int) {
		t := c.Theme()
		cell := ui.Box(c).Fill().MinWidth(0).Radius(t.Radius).TextColor(t.Text).Role(ui.RoleListItem)
		if s != nil {
			h := s.cells[i]
			if h == nil {
				slot := len(s.cells)
				if slot == len(s.handles) {
					s.handles = append(s.handles, &ui.Handle{})
				}
				h = s.handles[slot]
				s.cells[i] = h
			}
			cell.Bind(h)
			if s.Label != nil {
				cell.Label(s.Label(i))
			}
		}
		if interactive {
			cell.Selected(s.choice.chosen(i)).Background(ui.Transparent).TextColor(t.Text)
			if s.choice.chosen(i) {
				cell.Background(t.Text.Alpha(.1))
			}
			if i == s.choice.position() {
				cursor = cell
				grid.ActiveDescendant(cell)
			}
			if cell.DoubleClicked() {
				s.choice.setPosition(i, count)
				s.choice.submit(i, nil)
			}
			cell.OnClick(func() { s.Focus(c); s.choice.choose(i, count, cell.ClickModifiers()) })
		}
		cell.Children(func() {
			if item != nil {
				item(i)
			}
		})
	})
	if interactive {
		grid.Focusable().ActiveDescendant(cursor)
		collectionKeys(grid, &s.choice, count, nil, func() int { return s.measuredColumns(c) })
		if s.Label != nil {
			grid.HandleInput(func(event ui.InputEvent) bool {
				if event.Kind != ui.InputText || event.Text == "" || event.Mods&(ui.Ctrl|ui.Cmd|ui.Alt) != 0 {
					return false
				}
				s.choice.typeAhead(event.Text, event.Time, count)
				return true
			})
		}
	}
	return grid
}

func (s *GridState) visible() (first, last int) {
	first = -1
	for i := range s.cells {
		if first < 0 || i < first {
			first = i
		}
		last = max(last, i+1)
	}
	return max(0, first), last
}

// Use actual upstream cell geometry, not a second copy of its width/padding
// calculation. The first built row is complete unless it is the final row.
func (s *GridState) measuredColumns(c *ui.Context) int {
	first, _ := s.visible()
	h := s.cells[first]
	if h == nil {
		return max(1, s.columns)
	}
	r := h.Bounds(c)
	if r.W <= 0 {
		return max(1, s.columns)
	}
	for i := first + 1; ; i++ {
		next := s.cells[i]
		if next == nil {
			if first == 0 {
				return max(1, i-first)
			}
			return max(1, s.columns)
		}
		if next.Bounds(c).Y != r.Y {
			return max(1, i-first)
		}
	}
}
