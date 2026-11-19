// Collection behavior is adapted from MyGo v0.3.3 (MIT); see LICENSE.mygo.
package components

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// TreeVirtualizedState keeps keyed expansion, list selection and scrolling.
// Use one state per tree and do not copy it after first use. Keys must be unique
// throughout an acyclic tree. List.Selected optionally exposes the cursor;
// otherwise an internal cursor is used. List.Selection enables multi-selection.
type TreeVirtualizedState[K comparable] struct {
	List ListState
	Open ui.Selection[K]

	cursor int
	rows   []treeRow[K]
}

type treeRow[K comparable] struct {
	item          K
	depth, parent int
	branch        bool
}

// Rows reports the number of expanded rows in the last build.
func (s *TreeVirtualizedState[K]) Rows() int         { return len(s.rows) }
func (s *TreeVirtualizedState[K]) Item(row int) K    { return s.rows[row].item }
func (s *TreeVirtualizedState[K]) Depth(row int) int { return s.rows[row].depth }

func (s *TreeVirtualizedState[K]) flatten(roots []K, children func(K) []K) {
	s.rows = s.rows[:0]
	var walk func([]K, int, int)
	walk = func(items []K, depth, parent int) {
		for _, item := range items {
			kids := children(item)
			i := len(s.rows)
			s.rows = append(s.rows, treeRow[K]{item: item, depth: depth, parent: parent, branch: kids != nil})
			if kids != nil && s.Open.Has(item) {
				walk(kids, depth+1, i)
			}
		}
	}
	walk(roots, 0, -1)
}

func (s *TreeVirtualizedState[K]) setOpen(i int, open, recursive bool, children func(K) []K) {
	if i < 0 || i >= len(s.rows) || !s.rows[i].branch {
		return
	}
	if !open {
		// Keep the cursor visible when an ancestor is collapsed by its arrow.
		for j := s.List.position(); j > i && j < len(s.rows); j = s.rows[j].parent {
			if s.rows[j].parent == i {
				s.List.choose(i, len(s.rows), 0)
				break
			}
		}
	}
	var apply func(K)
	apply = func(item K) {
		if open {
			s.Open.Add(item)
		} else {
			s.Open.Remove(item)
		}
		if recursive {
			for _, child := range children(item) {
				if children(child) != nil {
					apply(child)
				}
			}
		}
	}
	apply(s.rows[i].item)
}

// TreeVirtualized builds Onevra tree rows in MyGo's virtual scrolling container.
// Give it a height or fill a bounded parent. children returns nil for leaves;
// row builds content after the arrow. Row builders may run during layout and
// must not perform application work. Only row UI is virtualized: expanded
// nodes are still traversed each frame.
//
// An arrow click or row double click toggles a branch. Enter activates the
// current row (and toggles branches); List.OnSubmit also reports activation.
// Left/Right close/open branches or select parents/children. Option on macOS,
// Shift elsewhere, with Left/Right applies expansion recursively. Alt-clicking
// an arrow does the same. All row styling is local; the frame theme is untouched.
func TreeVirtualized[K comparable](c *ui.Context, state *TreeVirtualizedState[K], roots []K, children func(K) []K, row func(K)) ui.Element {
	if state == nil {
		panic("components.TreeVirtualized: nil state")
	}
	if children == nil {
		children = func(K) []K { return nil }
	}
	if state.List.Selected == nil {
		state.List.Selected = &state.cursor
	}
	state.flatten(roots, children)
	state.List.Key = func(i int) any { return state.rows[i].item }
	toggle := func(i int) {
		state.setOpen(i, !state.Open.Has(state.rows[i].item), false, children)
	}
	e := collection(c, &state.List, len(state.rows), ui.RoleTree, func(i int, item ui.Element) {
		r := state.rows[i]
		item.Role(ui.RoleTreeItem).Level(r.depth + 1)
		if r.branch {
			item.Expanded(state.Open.Has(r.item))
		}
	}, toggle, func(i int) {
		r := state.rows[i]
		t := c.Theme()
		ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(1)).
			Padding(t.Space(1), t.Space(2), t.Space(1), t.Space(2+4*float32(r.depth))).Children(func() {
			treeDisclosureArrow(c, r.branch, state.Open.Has(r.item), func() {
				state.List.Focus(c)
				state.setOpen(i, !state.Open.Has(r.item), c.Modifiers()&ui.Alt != 0, children)
			})
			if row != nil {
				row(r.item)
			}
		})
	})
	recursiveModifier := ui.Shift
	if runtime.GOOS == "darwin" {
		recursiveModifier = ui.Alt
	}
	for _, mods := range []ui.Modifiers{0, recursiveModifier} {
		for _, key := range []ui.Key{ui.KeyLeft, ui.KeyRight} {
			e.OnShortcut(mods, key, func() {
				i := state.List.position()
				if i < 0 || i >= len(state.rows) {
					return
				}
				r := state.rows[i]
				open := state.Open.Has(r.item)
				switch {
				case key == ui.KeyRight && r.branch && (!open || mods != 0):
					state.setOpen(i, true, mods != 0, children)
				case key == ui.KeyRight && r.branch && open && i+1 < len(state.rows) && state.rows[i+1].parent == i:
					state.List.choose(i+1, len(state.rows), 0)
				case key == ui.KeyLeft && r.branch && open:
					state.setOpen(i, false, mods != 0, children)
				case key == ui.KeyLeft && r.parent >= 0:
					state.List.choose(r.parent, len(state.rows), 0)
				}
			})
		}
	}
	return e
}
