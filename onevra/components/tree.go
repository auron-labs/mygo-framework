// Collection behavior is adapted from MyGo v0.3.3 (MIT); see LICENSE.mygo.
package components

import "github.com/egoist/mygo/ui"

type (
	treeScopeKey struct{}
	treeScope    struct {
		depth, parent int
		items         []treeNode
	}
)

type treeNode struct {
	element ui.Element
	parent  int
	open    *bool
	branch  bool
}

// Tree builds a non-virtualized tree from Onevra rows and MyGo primitives.
// Applications own expansion and selection. Place it in a ScrollView when
// scrolling is needed. Up/Down and Home/End move focus; Left/Right navigate
// parents and children or collapse/expand branches. MyGo owns input dispatch,
// focus, layout and rendering, not the tree's appearance.
func Tree(c *ui.Context, children func()) ui.Element {
	e := ui.Column(c).FillWidth().MinWidth(0).Role(ui.RoleTree)
	// A context-local build scope, restored before returning, permits nested
	// trees without global state or retaining pass-local Elements across frames.
	slot := ui.Local(c.Root(), treeScopeKey{}, func() *treeScope { return nil })
	previous := *slot
	scope := &treeScope{parent: -1}
	*slot = scope
	defer func() { *slot = previous }()
	e.Children(children)
	for i, item := range scope.items {
		focus := func(j int) {
			if j >= 0 && j < len(scope.items) {
				scope.items[j].element.Focus().ScrollIntoView()
			}
		}
		item.element.OnShortcut(0, ui.KeyUp, func() { focus(i - 1) }).
			OnShortcut(0, ui.KeyDown, func() { focus(i + 1) }).
			OnShortcut(0, ui.KeyHome, func() { focus(0) }).
			OnShortcut(0, ui.KeyEnd, func() { focus(len(scope.items) - 1) }).
			OnShortcut(0, ui.KeyRight, func() {
				if item.branch && !*item.open {
					*item.open = true
				} else if item.branch && i+1 < len(scope.items) && scope.items[i+1].parent == i {
					focus(i + 1)
				}
			}).OnShortcut(0, ui.KeyLeft, func() {
			if item.branch && *item.open {
				*item.open = false
			} else {
				focus(item.parent)
			}
		})
	}
	return e
}

// TreeItem builds a text row and, when open, its nested TreeItems. A nil open
// or children makes a leaf. Clicking the arrow or double-clicking the row
// toggles expansion; Clicked lets the application select or activate the row.
// Selection uses the Sidebar's neutral tint, without a hover face.
func TreeItem(c *ui.Context, label string, open *bool, selected bool, children func()) ui.Element {
	return TreeItemWithLeading(c, label, open, selected, nil, children)
}

// TreeItemWithLeading is a TreeItem with an application-built leading slot
// between its disclosure arrow and label, for example a file type or host
// mark. A nil builder leaves no leading space. The builder receives the
// theme-scaled icon size as a suggestion and owns its content's size and
// appearance. Keep it non-interactive and free of I/O or background work.
func TreeItemWithLeading(c *ui.Context, label string, open *bool, selected bool, leading func(c *ui.Context, size float32), children func()) ui.Element {
	t := c.Theme()
	slot := ui.Local(c.Root(), treeScopeKey{}, func() *treeScope { return nil })
	scope := *slot
	if scope == nil {
		scope = &treeScope{parent: -1}
	}
	branch := open != nil && children != nil
	e := ui.ButtonBase(c).Key(label).Label(label).Role(ui.RoleTreeItem).
		Level(scope.depth+1).FillWidth().MinWidth(0).MinHeight(t.Space(7)).
		Justify(ui.Start).Gap(t.Space(.5)).Radius(t.Radius).FocusRing(true).
		Padding(0, t.Space(2), 0, t.Space(1+4*float32(scope.depth))).
		Selected(selected).Background(ui.Transparent).TextColor(t.Text)
	if selected {
		e.Background(t.Text.Alpha(.1))
	}
	if branch {
		e.Expanded(*open)
		if e.DoubleClicked() {
			*open = !*open
		}
	}
	index := len(scope.items)
	scope.items = append(scope.items, treeNode{element: e, parent: scope.parent, open: open, branch: branch})
	e.Children(func() {
		treeDisclosureArrow(c, branch, branch && *open, func() { *open = !*open })
		if leading != nil {
			// The mark sits nearer its label than the arrow does.
			ui.Box(c).Shrink(0).Margin(0, t.Space(1), 0, t.Space(.5)).Children(func() { leading(c, t.Space(3.5)) })
		}
		Text(c, label).SingleLine()
	})
	if branch && *open {
		parent := scope.parent
		scope.depth++
		scope.parent = index
		children()
		scope.parent = parent
		scope.depth--
	}
	return e
}

// The arrow is a separate pointer target, not another Tab stop. Its click does
// not activate/select the row, and leaves keyboard navigation with the tree.
func treeDisclosureArrow(c *ui.Context, branch, open bool, toggle func()) {
	t := c.Theme()
	arrow := ui.Box(c).Size(t.Space(4), t.Space(4)).Shrink(0).Role(ui.RoleNone)
	if !branch {
		return
	}
	arrow.OnClick(toggle).Draw(func(p *ui.Painter, r ui.Rect) {
		var path ui.Path
		if open {
			path.MoveTo(r.X+r.W/4, r.Y+r.H*3/8).LineTo(r.X+r.W/2, r.Y+r.H*5/8).LineTo(r.X+r.W*3/4, r.Y+r.H*3/8)
		} else {
			path.MoveTo(r.X+r.W*3/8, r.Y+r.H/4).LineTo(r.X+r.W*5/8, r.Y+r.H/2).LineTo(r.X+r.W*3/8, r.Y+r.H*3/4)
		}
		p.StrokePath(&path, 1.5, t.TextMuted)
	})
}
