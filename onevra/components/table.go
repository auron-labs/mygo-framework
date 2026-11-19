// Table behavior is adapted from MyGo v0.4.1 (MIT); see LICENSE.mygo.
package components

import (
	"maps"
	"slices"

	"github.com/egoist/mygo/ui"
)

// TableState keeps the column layout and sort order of a Table or
// TableVirtualized. Keep one state per table and do not copy it after first
// use. Columns holds the order and widths chosen by dragging headers, keyed by
// column ID, and can be persisted by the application. Sort, when set, lets a
// click on a Sortable header choose the order; the application sorts its rows
// and OnSort reports the user's change.
type TableState struct {
	Columns ui.TableLayout
	Sort    *ui.SortOrder
	OnSort  func()

	drag tableDrag
}

// tableDrag is a header press: the column by ID, its pointer offset and
// whether the drag has started moving the column.
type tableDrag struct {
	id     string
	dx     float32
	moving bool
}

// tableColumns is one build's arrangement of the application's columns.
type tableColumns struct {
	state   *TableState
	columns []ui.TableColumn
	order   []int
	least   float32
}

func tableColumnID(col *ui.TableColumn) string {
	if col.ID != "" {
		return col.ID
	}
	return col.Title
}

func newTableColumns(c *ui.Context, s *TableState, columns []ui.TableColumn) *tableColumns {
	l := &tableColumns{state: s, columns: columns}
	// Fixed columns keep their places; movable ones follow the layout order,
	// then the application's order.
	var movable []int
	for _, id := range s.Columns.Order {
		j := slices.IndexFunc(columns, func(col ui.TableColumn) bool { return tableColumnID(&col) == id })
		if j >= 0 && !columns[j].Fixed && !slices.Contains(movable, j) {
			movable = append(movable, j)
		}
	}
	for j := range columns {
		if !columns[j].Fixed && !slices.Contains(movable, j) {
			movable = append(movable, j)
		}
	}
	for j := range columns {
		if columns[j].Fixed {
			l.order = append(l.order, j)
		} else {
			l.order = append(l.order, movable[0])
			movable = movable[1:]
		}
	}
	for _, j := range l.order {
		if w, ok := l.width(&columns[j]); ok {
			l.least += w
		} else {
			l.least += l.flexMin(c, &columns[j])
		}
	}
	return l
}

func (l *tableColumns) flexMin(c *ui.Context, col *ui.TableColumn) float32 {
	return max(col.MinWidth, c.Theme().Space(15))
}

// width reports the user's or application's width, or false for a column
// sharing the remaining room.
func (l *tableColumns) width(col *ui.TableColumn) (float32, bool) {
	w := col.Width
	if v, ok := l.state.Columns.Widths[tableColumnID(col)]; ok && !col.Fixed {
		w = v
	}
	if w <= 0 {
		return 0, false
	}
	return clampTableWidth(col, w), true
}

func clampTableWidth(col *ui.TableColumn, w float32) float32 {
	if col.MaxWidth > 0 {
		w = min(w, col.MaxWidth)
	}
	return max(w, col.MinWidth, 16)
}

func (l *tableColumns) setWidth(col *ui.TableColumn, w float32) {
	// Clone rather than mutate a map the application may have restored.
	widths := maps.Clone(l.state.Columns.Widths)
	if widths == nil {
		widths = map[string]float32{}
	}
	widths[tableColumnID(col)] = clampTableWidth(col, w)
	l.state.Columns.Widths = widths
}

// cells builds a row's cells in display order. Shared flex bases keep
// variable cell content aligned with the header.
func (l *tableColumns) cells(c *ui.Context, role ui.Role, fill func(col int)) []ui.Element {
	t := c.Theme()
	boxes := make([]ui.Element, 0, len(l.order))
	for _, j := range l.order {
		col := &l.columns[j]
		box := ui.Row(c).Key(tableColumnID(col)).Padding(t.Space(1.5), t.Space(2.5)).
			AlignItems(ui.Center).Shrink(0).MinWidth(0).Clip().Role(role)
		if w, ok := l.width(col); ok {
			box.Width(w)
		} else {
			box.Basis(0).Grow(1).Shrink(1).MinWidth(l.flexMin(c, col))
		}
		switch col.Align {
		case ui.Center:
			box.Justify(ui.Center)
		case ui.End:
			box.Justify(ui.End)
		}
		box.Children(func() {
			if fill != nil {
				fill(j)
			}
		})
		boxes = append(boxes, box)
	}
	return boxes
}

// header builds the column titles. Sorting, moving and resizing respond to the
// pointer only, matching MyGo's table; disabled ancestors suppress them.
func (l *tableColumns) header(c *ui.Context) ui.Element {
	t := c.Theme()
	s := l.state
	head := ui.Row(c).FillWidth().MinHeight(t.Space(8)).Shrink(0).AlignItems(ui.Stretch).Role(ui.RoleRow).Clip()
	head.Children(func() {
		heads := l.cells(c, ui.RoleColumnHeader, func(j int) {
			col := &l.columns[j]
			sorted := s.Sort != nil && col.Sortable && s.Sort.Column == tableColumnID(col)
			title := Text(c, col.Title).SingleLine().FontWeight(600).TextColor(t.TextMuted)
			if sorted {
				title.TextColor(t.Text)
				tableSortArrow(c, s.Sort.Descending)
			}
		})
		for k, j := range l.order {
			l.headerInput(c, heads, k, &l.columns[j])
		}
	})
	ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.Border).Role(ui.RoleNone)
	return head
}

func (l *tableColumns) headerInput(c *ui.Context, heads []ui.Element, k int, col *ui.TableColumn) {
	t := c.Theme()
	s, h, id := l.state, heads[k], tableColumnID(col)
	enabled := !h.IsDisabled()
	moved, held, dx := false, false, float32(0)
	if enabled && !col.Fixed {
		dx, _, held = h.Dragged()
	}
	if held {
		if s.drag.id != id {
			s.drag = tableDrag{id: id}
		}
		s.drag.dx += dx
		s.drag.moving = s.drag.moving || max(s.drag.dx, -s.drag.dx) > 4
		if s.drag.moving {
			l.move(heads)
			h.Left(s.drag.dx).Background(t.Text.Alpha(.1)).Opacity(.9)
		}
	} else if s.drag.id == id && !held {
		// The release frame also reports the click that ends a move.
		moved = s.drag.moving
		s.drag = tableDrag{}
	}
	if s.Sort != nil && col.Sortable && h.Clicked() && !moved {
		if s.Sort.Column == id {
			s.Sort.Descending = !s.Sort.Descending
		} else {
			*s.Sort = ui.SortOrder{Column: id}
		}
		if s.OnSort != nil {
			s.OnSort()
		}
	}
	if col.Fixed {
		return
	}
	h.Children(func() {
		edge := ui.Box(c).Absolute().Top(0).Bottom(0).Right(0).Width(t.Space(2)).Role(ui.RoleNone)
		if enabled {
			edge.Cursor(ui.CursorResizeEW)
		}
		if dx, _, held := edge.Dragged(); held && enabled && dx != 0 {
			// Earlier flexible columns keep their widths so the edge follows
			// the pointer; later columns or sideways scrolling absorb the rest.
			for m, j := range l.order[:k] {
				if _, ok := l.width(&l.columns[j]); !ok {
					l.setWidth(&l.columns[j], heads[m].LayoutBounds().W)
				}
			}
			w, ok := l.width(col)
			if !ok {
				w = h.LayoutBounds().W
			}
			l.setWidth(col, w+dx)
		}
		// Read the state while building: paint-only passes, such as an exiting
		// Dialog's copy, cannot use the handle.
		color := t.Border
		if enabled && (edge.Hovered() || edge.Pressed()) {
			color = t.TextMuted
		}
		edge.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X + r.W - 1, Y: r.Y + r.H/4, W: 1, H: r.H / 2}, color, 0)
		})
	})
}

// move swaps the dragged column with a movable neighbor whose middle it has
// passed, keeping the header under the pointer.
func (l *tableColumns) move(heads []ui.Element) {
	s := l.state
	at := slices.IndexFunc(l.order, func(j int) bool { return tableColumnID(&l.columns[j]) == s.drag.id })
	if at < 0 {
		return
	}
	to := at
	switch {
	case s.drag.dx > 0 && at+1 < len(l.order):
		if next := heads[at+1].LayoutBounds(); next.W > 0 && s.drag.dx > next.W/2 {
			to = at + 1
		}
	case s.drag.dx < 0 && at > 0:
		if prev := heads[at-1].LayoutBounds(); prev.W > 0 && -s.drag.dx > prev.W/2 {
			to = at - 1
		}
	}
	if to == at || l.columns[l.order[to]].Fixed {
		return
	}
	if to > at {
		s.drag.dx -= heads[to].LayoutBounds().W
	} else {
		s.drag.dx += heads[to].LayoutBounds().W
	}
	ids := make([]string, len(l.order))
	for k, j := range l.order {
		ids[k] = tableColumnID(&l.columns[j])
	}
	ids[at], ids[to] = ids[to], ids[at]
	s.Columns.Order = ids
}

func tableSortArrow(c *ui.Context, descending bool) {
	t := c.Theme()
	ui.Box(c).Size(t.Space(2.5), t.Space(2.5)).Shrink(0).Margin(0, 0, 0, t.Space(1)).Role(ui.RoleNone).Draw(func(p *ui.Painter, r ui.Rect) {
		var path ui.Path
		if descending {
			path.MoveTo(r.X+r.W*.1, r.Y+r.H*.3).LineTo(r.X+r.W*.5, r.Y+r.H*.7).LineTo(r.X+r.W*.9, r.Y+r.H*.3)
		} else {
			path.MoveTo(r.X+r.W*.1, r.Y+r.H*.7).LineTo(r.X+r.W*.5, r.Y+r.H*.3).LineTo(r.X+r.W*.9, r.Y+r.H*.7)
		}
		p.StrokePath(&path, 1.5, t.TextMuted)
	})
}

// Odd rows carry a faint neutral stripe, as in macOS tables; selection
// replaces it with the stronger tint. Neither depends on hover.
func tableRowBackground(c *ui.Context, row ui.Element, i int, selected bool) {
	t := c.Theme()
	switch {
	case selected:
		row.Background(t.Text.Alpha(.1))
	case i%2 == 1:
		row.Background(t.Text.Alpha(.025))
	}
}

type (
	tableScopeKey struct{}
	tableScope    struct {
		layout *tableColumns
		rows   []ui.Element
	}
)

// Table builds a non-virtualized table of TableRows under a header of columns.
// Applications own rows, selection and sorting; a nil state keeps the column
// layout on the table without sorting. The table is at least as wide as its
// columns; place it in a ScrollView when scrolling is needed. Up/Down and
// Home/End move focus between rows. Dragging a header moves its column and
// dragging its right edge resizes it, unless the column is Fixed.
func Table(c *ui.Context, state *TableState, columns []ui.TableColumn, rows func()) ui.Element {
	e := ui.Column(c).FillWidth().Role(ui.RoleTable).Clip()
	if state == nil {
		state = ui.Local(e, "table", func() TableState { return TableState{} })
	}
	layout := newTableColumns(c, state, columns)
	e.MinWidth(layout.least)
	// As in Tree, a restored build scope permits nested tables without
	// retaining pass-local Elements across frames.
	slot := ui.Local(c.Root(), tableScopeKey{}, func() *tableScope { return nil })
	previous := *slot
	scope := &tableScope{layout: layout}
	*slot = scope
	defer func() { *slot = previous }()
	e.Children(func() {
		layout.header(c)
		if rows != nil {
			rows()
		}
	})
	for i, row := range scope.rows {
		focus := func(j int) {
			if j >= 0 && j < len(scope.rows) {
				scope.rows[j].Focus().ScrollIntoView()
			}
		}
		row.OnShortcut(0, ui.KeyUp, func() { focus(i - 1) }).
			OnShortcut(0, ui.KeyDown, func() { focus(i + 1) }).
			OnShortcut(0, ui.KeyHome, func() { focus(0) }).
			OnShortcut(0, ui.KeyEnd, func() { focus(len(scope.rows) - 1) })
	}
	return e
}

// TableRow builds a row of a Table, cell building the content of each column
// by its index in the Table's columns. key identifies the row among its
// siblings and label names it for assistive technology. Clicked lets the
// application select or activate the row. Rows alternate with a faint stripe
// and selection uses the Tree's neutral tint, without a hover face. Call it
// only while building a Table's rows.
func TableRow(c *ui.Context, key any, label string, selected bool, cell func(col int)) ui.Element {
	t := c.Theme()
	scope := *ui.Local(c.Root(), tableScopeKey{}, func() *tableScope { return nil })
	if scope == nil {
		panic("components.TableRow: outside Table")
	}
	e := ui.ButtonBase(c).Key(key).Label(label).Role(ui.RoleRow).FillWidth().MinWidth(0).
		MinHeight(t.Space(8)).Justify(ui.Start).AlignItems(ui.Stretch).Gap(0).
		Radius(t.Radius).FocusRing(true).Selected(selected).Background(ui.Transparent).TextColor(t.Text)
	tableRowBackground(c, e, len(scope.rows), selected)
	scope.rows = append(scope.rows, e)
	e.Children(func() { scope.layout.cells(c, ui.RoleCell, cell) })
	return e
}
