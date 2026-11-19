// Table behavior is adapted from MyGo v0.4.1 (MIT); see LICENSE.mygo.
package components

import "github.com/egoist/mygo/ui"

// TableVirtualizedState keeps a virtualized table's column layout, sort order,
// row selection and scrolling. Use one state per table and do not copy it after
// first use. List follows ListState's selection and activation contract: a nil
// List.Selected and List.Selection disables selection.
type TableVirtualizedState struct {
	TableState
	List ListState
}

// TableVirtualized builds Onevra table rows in MyGo's virtual scrolling
// container, under a header that scrolls sideways with them when the columns
// are wider than the table. Give it a height or fill a bounded parent. cell
// builds the content of a row's column, by its index in columns; it may run
// during layout and must not perform application work. Rows are at least as
// high as the header, grow to their tallest cell and alternate with a faint
// stripe, as in Table.
//
// Clicks, Up/Down, Home/End and Page Up/Down choose rows; Enter and double
// clicks report List.OnSubmit. Header dragging, resizing and sorting follow
// Table. All row styling is local; the frame theme is untouched.
func TableVirtualized(c *ui.Context, state *TableVirtualizedState, columns []ui.TableColumn, count int, cell func(row, col int)) ui.Element {
	if state == nil {
		panic("components.TableVirtualized: nil state")
	}
	t := c.Theme()
	layout := newTableColumns(c, &state.TableState, columns)
	e := ui.Column(c).Role(ui.RoleTable).Clip()
	e.Children(func() {
		head := layout.header(c)
		list := collection(c, &state.List, count, ui.RoleNone, func(i int, item ui.Element) {
			item.Role(ui.RoleRow)
			tableRowBackground(c, item, i, state.List.chosen(i))
		}, nil, func(i int) {
			ui.Row(c).FillWidth().MinWidth(0).MinHeight(t.Space(8)).AlignItems(ui.Stretch).Children(func() {
				layout.cells(c, ui.RoleCell, func(j int) {
					if cell != nil {
						cell(i, j)
					}
				})
			})
		}).Grow(1).MinHeight(0).RowWidth(layout.least)
		head.ScrollWith(list)
	})
	return e
}
