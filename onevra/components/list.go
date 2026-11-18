// Collection behavior is adapted from MyGo v0.3.3 (MIT); see LICENSE.mygo.
package components

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// ListState owns a Onevra list's selection and MyGo's virtual scroll state.
// Keep one state per list and do not copy it after first use. Keys must be
// comparable and unique. Without Key, selection identifies rows by index.
// Selection uses ui.Selection[any] so keys can have any comparable type.
// Selection, when set, controls the chosen rows; Selected optionally exposes
// the keyboard cursor as well. A nil Selected and Selection disables selection.
// KeepOffset keeps a keyed list's scroll offset as its rows move, such as when
// they are sorted again, while selection still follows its keys.
//
// OnChange and OnSubmit report user selection and activation respectively;
// use these callbacks rather than the returned Element's Changed/Submitted.
// MyGo still owns virtualization, variable-height measurement and scrolling.
// This state intentionally does not expose table sorting, reordering or headers.
type ListState struct {
	Selected   *int
	Selection  *ui.Selection[any]
	Key        func(int) any
	Label      func(int) string
	KeepOffset bool
	FollowEnd  bool
	OnChange   func()
	OnSubmit   func(int)

	// Optional adapters let grid selection reuse collection behavior while
	// MyGo's grid owns scrolling and visible cell geometry.
	reveal    func(int)
	visible   func() (int, int)
	scroll    ui.ListState
	cursor    int
	anchor    int
	last      int
	key       any
	keySet    bool
	anchorKey any
	anchorSet bool
	typed     string
	typedAt   time.Time
}

func (s *ListState) ScrollTo(row int, align ui.Align) { s.scroll.ScrollTo(row, align) }
func (s *ListState) ScrollIntoView(row int) {
	if s.reveal != nil {
		s.reveal(row)
		return
	}
	s.scroll.ScrollIntoView(row)
}
func (s *ListState) ScrollToEnd() { s.scroll.ScrollToEnd() }
func (s *ListState) Visible() (first, last int) {
	if s.visible != nil {
		return s.visible()
	}
	return s.scroll.Visible()
}
func (s *ListState) Focus(c *ui.Context) bool       { return s.scroll.Focus(c) }
func (s *ListState) Focused(c *ui.Context) bool     { return s.scroll.Focused(c) }
func (s *ListState) FocusWithin(c *ui.Context) bool { return s.scroll.FocusWithin(c) }

// List builds neutral, hover-free rows inside MyGo's non-selecting virtual list.
// Give it a height or fill a bounded parent. Applications own row content and
// spacing; builders can run during layout and must not perform application work.
// A nil state uses MyGo's local scrolling state, without selection.
func List(c *ui.Context, state *ListState, count int, row func(int)) ui.Element {
	return collection(c, state, count, ui.RoleList, nil, nil, row)
}

// collection uses the upstream list only for layout and scrolling. Its private
// ListState never enables upstream selection, so no styled row face is created,
// including when the layout engine builds additional rows after construction.
func collection(c *ui.Context, s *ListState, count int, role ui.Role, decorate func(int, ui.Element), activate func(int), row func(int)) ui.Element {
	count = max(0, count)
	var scroll *ui.ListState
	if s != nil {
		s.sync(count)
		s.scroll.Key, s.scroll.KeepOffset, s.scroll.FollowEnd = s.Key, s.KeepOffset, s.FollowEnd
		scroll = &s.scroll
	}
	e := ui.List(c, scroll, count).Role(role)
	interactive := s != nil && (s.Selected != nil || s.Selection != nil)
	if interactive {
		e.Focusable()
		listKeys(e, s, count, activate)
		if s.Label != nil {
			e.HandleInput(func(event ui.InputEvent) bool {
				if event.Kind != ui.InputText || event.Text == "" || event.Mods&(ui.Ctrl|ui.Cmd|ui.Alt) != 0 {
					return false
				}
				s.typeAhead(event.Text, event.Time, count)
				return true
			})
		}
	}
	return e.Rows(func(r ui.ListRow) {
		c := r.Context
		i := r.Index
		t := c.Theme()
		item := ui.Box(c).FillWidth().MinWidth(0).Radius(t.Radius).TextColor(t.Text).Role(ui.RoleListItem)
		if interactive {
			selected := s.chosen(i)
			if i == s.position() {
				e.ActiveDescendant(item)
			}
			item.Selected(selected).Background(ui.Transparent).TextColor(t.Text)
			if selected {
				item.Background(t.Text.Alpha(.1))
			}
			// MyGo reports double-click on the second press, before the
			// release that produces Clicked; it is not an OnClick sub-event.
			if item.DoubleClicked() {
				s.setPosition(i, count)
				s.submit(i, activate)
			}
			item.OnClick(func() {
				s.Focus(c)
				s.choose(i, count, item.ClickModifiers())
			})
		}
		if s != nil && s.Label != nil {
			item.Label(s.Label(i))
		}
		if decorate != nil {
			decorate(i, item)
		}
		item.Children(func() {
			if row != nil {
				row(i)
			}
		})
	})
}

func (s *ListState) position() int {
	if s.Selected != nil {
		return *s.Selected
	}
	return s.cursor
}

func (s *ListState) itemKey(i int) any {
	if s.Key != nil {
		return s.Key(i)
	}
	return i
}

func (s *ListState) chosen(i int) bool {
	if s.Selection != nil {
		return s.Selection.Has(s.itemKey(i))
	}
	return s.Selected != nil && *s.Selected == i
}

func (s *ListState) setPosition(i, count int) {
	s.cursor = i
	if s.Selected != nil {
		*s.Selected = i
	}
	s.last, s.keySet = i, i >= 0 && i < count
	if s.keySet {
		s.key = s.itemKey(i)
	} else {
		s.key = nil
	}
}

// Keep a keyed cursor with its item when rows are inserted or removed. An
// explicit application change to Selected takes precedence over reconciliation.
func (s *ListState) sync(count int) {
	i := s.position()
	if i != s.last {
		s.setAnchor(i, count)
	}
	if s.Key != nil && s.keySet && i == s.last && (i >= count || i < 0 || s.itemKey(i) != s.key) {
		i = -1
		for j := 0; j < count; j++ {
			if s.itemKey(j) == s.key {
				i = j
				break
			}
		}
	}
	if s.Key != nil && s.anchorSet && (s.anchor < 0 || s.anchor >= count || s.itemKey(s.anchor) != s.anchorKey) {
		s.anchor = i
		for j := 0; j < count; j++ {
			if s.itemKey(j) == s.anchorKey {
				s.anchor = j
				break
			}
		}
		s.setAnchor(s.anchor, count)
	}
	if i >= count {
		i = count - 1
	}
	s.setPosition(max(-1, i), count)
}

func (s *ListState) choose(i, count int, mods ui.Modifiers) {
	if i < 0 || i >= count {
		return
	}
	changed := s.position() != i
	if selection := s.Selection; selection != nil {
		switch {
		case mods&ui.Shift != 0:
			anchor := max(0, min(s.anchor, count-1))
			if mods&selectionModifier() == 0 {
				selection.Clear()
			}
			for j := min(anchor, i); j <= max(anchor, i); j++ {
				selection.Add(s.itemKey(j))
			}
		case mods&selectionModifier() != 0:
			key := s.itemKey(i)
			if selection.Has(key) {
				selection.Remove(key)
			} else {
				selection.Add(key)
			}
			s.setAnchor(i, count)
		default:
			changed = changed || selection.Len() != 1 || !selection.Has(s.itemKey(i))
			selection.Clear()
			selection.Add(s.itemKey(i))
			s.setAnchor(i, count)
		}
		if mods&(ui.Shift|selectionModifier()) != 0 {
			changed = true
		}
	} else {
		s.setAnchor(i, count)
	}
	s.setPosition(i, count)
	s.ScrollIntoView(i)
	if changed && s.OnChange != nil {
		s.OnChange()
	}
}

func (s *ListState) setAnchor(i, count int) {
	s.anchor, s.anchorSet = i, i >= 0 && i < count
	if s.anchorSet {
		s.anchorKey = s.itemKey(i)
	} else {
		s.anchorKey = nil
	}
}

func (s *ListState) typeAhead(text string, now time.Time, count int) {
	text = strings.ToLower(text)
	start := s.position()
	if now.Sub(s.typedAt) > time.Second || s.typed == text {
		s.typed = text
		start++
	} else {
		s.typed += text
	}
	s.typedAt = now
	for offset := 0; offset < count; offset++ {
		i := (max(0, start) + offset) % count
		if strings.HasPrefix(strings.ToLower(s.Label(i)), s.typed) {
			s.choose(i, count, 0)
			return
		}
	}
}

func selectionModifier() ui.Modifiers {
	if runtime.GOOS == "darwin" {
		return ui.Cmd
	}
	return ui.Ctrl
}

func (s *ListState) submit(i int, activate func(int)) {
	if activate != nil {
		activate(i)
	}
	if s.OnSubmit != nil {
		s.OnSubmit(i)
	}
}

func listKeys(e ui.Element, s *ListState, count int, activate func(int)) {
	collectionKeys(e, s, count, activate, nil)
}

func collectionKeys(e ui.Element, s *ListState, count int, activate func(int), columns func() int) {
	keys := []ui.Key{ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown}
	if columns != nil {
		keys = append(keys, ui.KeyLeft, ui.KeyRight)
	}
	modifiers := []ui.Modifiers{0, ui.Shift}
	if runtime.GOOS != "darwin" && s.Selection != nil {
		modifiers = append(modifiers, ui.Ctrl, ui.Ctrl|ui.Shift)
		e.OnShortcut(ui.Ctrl, ui.KeySpace, func() { s.choose(s.position(), count, ui.Ctrl) })
	}
	for _, mods := range modifiers {
		for _, key := range keys {
			e.OnShortcut(mods, key, func() {
				i := s.position()
				first, last := s.Visible()
				page := max(1, last-first)
				stride := 1
				if columns != nil {
					stride = max(1, columns())
				}
				switch key {
				case ui.KeyLeft:
					i--
				case ui.KeyRight:
					i++
				case ui.KeyUp:
					i -= stride
				case ui.KeyDown:
					i += stride
				case ui.KeyHome:
					i = 0
				case ui.KeyEnd:
					i = count - 1
				case ui.KeyPageUp:
					i -= page
				case ui.KeyPageDown:
					i += page
				}
				i = max(0, min(i, count-1))
				if count == 0 {
					return
				}
				if mods == ui.Ctrl && s.Selection != nil && runtime.GOOS != "darwin" {
					changed := s.position() != i
					s.setPosition(i, count)
					s.ScrollIntoView(i)
					if changed && s.OnChange != nil {
						s.OnChange()
					}
				} else {
					s.choose(i, count, mods)
				}
			})
		}
	}
	e.OnShortcut(0, ui.KeyEnter, func() {
		i := s.position()
		if i >= 0 && i < count {
			s.submit(i, activate)
		}
	})
	if s.Selection != nil {
		e.OnShortcut(selectionModifier(), ui.KeyA, func() {
			changed := false
			for i := 0; i < count; i++ {
				changed = changed || !s.Selection.Has(s.itemKey(i))
				s.Selection.Add(s.itemKey(i))
			}
			if changed && s.OnChange != nil {
				s.OnChange()
			}
		})
	}
}
