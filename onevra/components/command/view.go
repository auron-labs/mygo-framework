package command

import (
	"time"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// CommandView builds search and grouped virtual results in a bounded parent,
// independently of any dialog. Expanded results fill that parent; collapsed search uses only
// the input's height. The host supplies the available size. Up/Down keep
// focus in the search field and skip disabled commands, stopping at either end.
// MyGo owns text editing and IME handling. Tab follows normal focus navigation.
// Stable keyed parents distinguish independently built views. The view paints
// no background, so the host's surface or window material shows through.
func CommandView(c *ui.Context, s *State, items []Item, options Options) ui.Element {
	t := c.Theme()
	root := ui.Column(c).FillWidth().MinWidth(0).MinHeight(0)
	root.Children(func() {
		input := components.TextBox(c, &s.Query, components.TextBoxOptions{}).
			Label(fallback(options.SearchLabel, l10n.In(c.Locale()).Text("searchCommands"))).Placeholder(fallback(options.Placeholder, l10n.In(c.Locale()).Text("searchCommandsPlaceholder"))).
			FillWidth().Shrink(0).FontSize(t.Rem(1.15)).
			Padding(t.Space(4.5), t.Space(4.5), t.Space(4.5), t.Space(12)).
			Background(ui.Transparent).Border(0, ui.Transparent).Radius(0).FocusRing(false)
		input.DrawOver(func(p *ui.Painter, r ui.Rect) {
			size := t.Space(5)
			p.Icon(lucide.Search(), ui.Rect{X: r.X + t.Space(4.5), Y: r.Y + (r.H-size)/2, W: size, H: size}, t.TextMuted)
		})
		// Resolve bound input before deriving results for this frame.
		input.Changed()
		s.composing = input.Composing()
		if s.refocus {
			input.Focus()
			s.refocus = false
		}
		if !s.inputSource.attempted && options.InputSource != "" {
			if !s.inputSource.waited {
				// Modal focus and the native input context are committed after
				// this build. Select on a subsequent frame, not during creation.
				s.inputSource.waited = true
				c.After(time.Millisecond)
			} else if input.Focused() && !s.composing {
				s.InputSourceError = s.inputSource.begin(options.InputSource, options.KeepInputSource, selectInputSource)
			}
		}
		if !resultsExpanded(s, options) {
			s.sync(nil)
			return
		}
		root.FillHeight()
		results := items
		if !options.PreFiltered {
			filter := options.Filter
			if filter == nil {
				filter = Filter
			}
			results = filter(items, s.Query)
		}
		results = grouped(results)
		if options.Loading || options.Error != "" {
			results = nil
		}
		s.sync(results)
		activate := func(id string) {
			for _, item := range results {
				if item.ID == id && id != "" && !item.Disabled {
					s.ActiveID = id
					if options.OnActivate != nil {
						options.OnActivate(id)
					}
					return
				}
			}
		}
		input.OnShortcut(0, ui.KeyDown, func() { s.move(results, 1) })
		input.OnShortcut(0, ui.KeyUp, func() { s.move(results, -1) })
		input.OnSubmit(func() { activate(s.ActiveID) })
		if len(results) == 0 {
			message := fallback(options.EmptyText, l10n.In(c.Locale()).Text("noMatchingCommands"))
			if options.Loading {
				message = fallback(options.LoadingText, l10n.In(c.Locale()).Text("loading"))
			}
			if options.Error != "" {
				message = options.Error
			}
			ui.Box(c).Fill().MinHeight(0).Padding(t.Space(4)).Center().Children(func() {
				components.Text(c, message).Role(ui.RoleStatus).TextColor(t.TextMuted).Wrap()
			})
		} else {
			s.scroll.Key = func(i int) any { return results[i].ID }
			components.List(c, &s.scroll, len(results), func(i int) {
				item := results[i]
				if item.Group != "" && (i == 0 || results[i-1].Group != item.Group) {
					components.Text(c, item.Group).TextColor(t.TextMuted).FontSize(t.Rem(.8)).Padding(t.Space(2), t.Space(2.5)).SingleLine()
				}
				row := ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(2.5)).
					Padding(t.Space(2.5), t.Space(2.5)).Radius(t.Radius * 1.5).Role(ui.RoleButton).
					Label(item.Title).Description(item.Description).Disabled(item.Disabled || item.ID == "").Selected(item.ID == s.ActiveID).TextColor(t.Text)
				if item.ID == s.ActiveID {
					row.Background(t.Text.Alpha(.1))
					input.ActiveDescendant(row)
				} else if row.Hovered() && !item.Disabled {
					row.Background(t.Text.Alpha(.04))
				}
				row.OnClick(func() { s.refocus = true; activate(item.ID) })
				row.Children(func() {
					if item.Leading != nil {
						item.Leading(c, t.Space(5))
					} else if item.Icon != nil {
						components.Icon(c, item.Icon).Size(t.Space(5), t.Space(5)).Shrink(0)
					}
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						color := t.Text
						if item.Disabled {
							color = t.TextMuted
						}
						components.Text(c, item.Title).TextColor(color).SingleLine()
						if item.Description != "" {
							components.Text(c, item.Description).TextColor(t.TextMuted).FontSize(t.Rem(.85)).SingleLine()
						}
					})
					if item.Trailing != nil {
						item.Trailing(c)
					}
					if item.Shortcut != "" {
						components.Text(c, item.Shortcut).TextColor(t.TextMuted).FontSize(t.Rem(.8)).Shrink(0).SingleLine()
					}
				})
			}).Label(fallback(options.ResultsLabel, l10n.In(c.Locale()).Text("commands"))).Fill().MinHeight(0).Padding(0, t.Space(2), t.Space(3), t.Space(2))
		}
	})
	return root
}
