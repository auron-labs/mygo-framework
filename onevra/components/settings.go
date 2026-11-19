package components

import (
	"strings"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// SettingsState is application-owned presentation state, independent of setting values.
// An empty or unknown Category selects the first category. Searching preserves Category.
type SettingsState struct {
	Category string
	Query    string
}

// SettingsCategory describes a navigation destination. IDs must be nonempty and
// unique among siblings; keep them stable across localization and rebuilds.
// Image, such as a project logo, replaces Icon in the navigation when set.
type SettingsCategory struct {
	ID, Title, Section, Description string
	Icon                            *ui.SVG
	Image                           ui.ImageSource
	// Leading replaces Image and Icon with application-built, non-interactive
	// sidebar content. It follows SidebarItemWithLeading's builder contract
	// and is not built in the narrow category selector or during metadata search.
	Leading func(c *ui.Context, size float32)
	// Placeholder, when set, shows application-built, non-interactive
	// content, such as an empty section's note, in the sidebar in place of
	// the category's navigation item, keeping its Section's heading and
	// actions. Such a category has no page and is never selected, searched
	// or offered by the narrow category selector. It follows Build's rules.
	Placeholder func(c *ui.Context)
	Groups      []SettingsGroup
}

// SettingsGroup groups related items. Title may be empty for an unheaded group.
// Actions optionally builds trailing controls on the group's header row, such
// as a refresh button; like Build, it runs on the UI thread only when the
// header is visible and must not start background work during rendering.
// Actions are not searched.
type SettingsGroup struct {
	ID, Title string
	Items     []SettingsItem
	Actions   func(c *ui.Context)
}

// SettingsItem describes a searchable row. Build runs synchronously on the UI
// thread only when visible. Applications own values, accessible control labels,
// persistence and effects; Build must not start background work during rendering.
// IDs must be nonempty and unique within the group.
//
// DescriptionContent optionally replaces Description's text with inline text
// elements, such as Text and a ButtonLink, built into the description's
// paragraph and styled like it. Only text elements belong in it; like Build,
// it runs only when visible. Description is still what search matches, so
// give it the same words.
type SettingsItem struct {
	ID, Title, Description string
	Keywords               []string
	Build                  func(c *ui.Context)
	DescriptionContent     func(c *ui.Context)
	// Leading builds content before the title and description, such as an
	// icon, avatar or status indicator.
	Leading func(c *ui.Context)
	// Stacked places Build below the text across the row's width, for wide
	// controls such as text boxes, sliders and option lists. Narrow layouts
	// always stack.
	Stacked bool
	// Content replaces the whole row with application-built content spanning
	// its width, such as a profile card, preview, banner or list; Leading,
	// Build and the displayed title and description are not used. Title,
	// Description and Keywords remain the row's search metadata. Like Build,
	// it runs only when visible.
	Content func(c *ui.Context)
}

// SettingsOptions controls presentation without imposing a window or dialog.
// Empty text fields use Onevra's catalog in the window's language. Narrow is supplied by the host from
// its allocated width (not the window width); it replaces the sidebar with a
// category selector and stacks each row's control below its text.
type SettingsOptions struct {
	Narrow                                                                        bool
	Title, SearchLabel, SearchPlaceholder, CategoryLabel, ResultsTitle, EmptyText string
	// HeaderActions builds optional trailing actions beside the close button.
	HeaderActions func(c *ui.Context)
	// SectionActions builds optional trailing controls on a section's
	// sidebar heading, such as an add button, for each nonempty Section; it
	// may build nothing for a section. The narrow category selector shows the
	// selected category's section actions beside it, so there a section of
	// only Placeholder categories has none. It follows Build's rules.
	// A section shows only while a category belongs to it.
	SectionActions func(c *ui.Context, section string)
	// Close, when set, shows a trailing close button that calls it, for hosts
	// that present Settings in a dialog or window. CloseLabel overrides its
	// accessible name; empty uses Onevra's catalog. Nil omits the button.
	Close      func()
	CloseLabel string
	// HideSearch removes the search field; Query is then ignored.
	HideSearch bool
	// HideSidebar removes category navigation, both the sidebar and the narrow
	// category selector, and shows every category on one page, each under its
	// title when there are several. To navigate from application UI instead,
	// pass only the selected category.
	HideSidebar bool
}

// SettingsResult identifies an item by slice indices in the supplied catalog.
// Results are valid only for that catalog snapshot and preserve declaration order.
type SettingsResult struct{ Category, Group, Item int }

// SearchSettings searches every category using case-insensitive, whitespace-
// separated terms. Every term must occur in the combined category, group, title,
// description or keywords. An empty query returns all items. No builders run.
func SearchSettings(categories []SettingsCategory, query string) []SettingsResult {
	terms := strings.Fields(strings.ToLower(query))
	var results []SettingsResult
	for ci, category := range categories {
		if category.Placeholder != nil {
			continue
		}
		for gi, group := range category.Groups {
			for ii, item := range group.Items {
				haystack := strings.ToLower(strings.Join([]string{category.Title, category.Section, category.Description, group.Title, item.Title, item.Description, strings.Join(item.Keywords, " ")}, " "))
				match := true
				for _, term := range terms {
					if !strings.Contains(haystack, term) {
						match = false
						break
					}
				}
				if match {
					results = append(results, SettingsResult{ci, gi, ii})
				}
			}
		}
	}
	return results
}

// Settings builds a bounded, scrollable settings surface using application-owned
// state and definitions. Place it in a parent with a finite height. It fills its
// parent, and can be embedded in Dialog, Shell content, or a standalone window.
// The host supplies the background or material; navigation adds a subtle tint.
// Use a keyed parent to distinguish multiple instances. state must not be nil.
// Navigation clears Query; clearing Query alone restores the selected category.
func Settings(c *ui.Context, state *SettingsState, categories []SettingsCategory, opts SettingsOptions) ui.Element {
	words := localized(c)
	if state == nil {
		panic("components.Settings: nil state")
	}
	if opts.Title == "" {
		opts.Title = words.Settings
	}
	if opts.SearchLabel == "" {
		opts.SearchLabel = words.SearchSettings
	}
	if opts.SearchPlaceholder == "" {
		opts.SearchPlaceholder = words.SearchSettingsPlaceholder
	}
	if opts.CategoryLabel == "" {
		opts.CategoryLabel = words.SettingsCategory
	}
	if opts.ResultsTitle == "" {
		opts.ResultsTitle = words.SearchResults
	}
	if opts.EmptyText == "" {
		opts.EmptyText = words.NoSettings
	}
	if opts.CloseLabel == "" {
		opts.CloseLabel = words.CloseSettings
	}
	selected := -1
	for i, category := range categories {
		if category.ID == state.Category && category.Placeholder == nil {
			selected = i
			break
		}
	}
	for i := 0; selected < 0 && i < len(categories); i++ {
		if categories[i].Placeholder == nil {
			selected = i
			state.Category = categories[i].ID
		}
	}
	t := c.Theme()
	search := func() {
		TextBox(c, &state.Query, TextBoxOptions{LeadingIcon: lucide.Search()}).Label(opts.SearchLabel).Placeholder(opts.SearchPlaceholder).FillWidth().MinWidth(0)
	}
	query := state.Query
	if opts.HideSearch {
		query = ""
	}
	searching := strings.TrimSpace(query) != ""
	// all shows every category: search results, or a page without navigation.
	all := searching || opts.HideSidebar
	heading := func(title string, actions bool) {
		ui.Row(c).FillWidth().MinHeight(t.Space(14)).Shrink(0).Padding(t.Space(3), t.Space(4)).Gap(t.Space(2)).Children(func() {
			Text(c, title).Bold()
			ui.Spacer(c)
			if actions {
				if opts.HeaderActions != nil {
					opts.HeaderActions(c)
				}
				if opts.Close != nil {
					if IconButton(c, lucide.X(), opts.CloseLabel, IconButtonPlain).Clicked() {
						opts.Close()
					}
				}
			}
		})
	}
	return ui.Row(c).Fill().MinWidth(0).Gap(0).Children(func() {
		if !opts.Narrow && !opts.HideSidebar {
			ui.Column(c).Width(t.Space(54)).FillHeight().Shrink(0).Background(t.Text.Alpha(.035)).Children(func() {
				heading(opts.Title, false)
				if !opts.HideSearch {
					ui.Column(c).FillWidth().Padding(0, t.Space(3), t.Space(4), t.Space(3)).Children(search)
				}
				ScrollView(c, ScrollViewOptions{}).Fill().Children(func() {
					Sidebar(c).Children(func() {
						for start := 0; start < len(categories); {
							section := categories[start].Section
							end := start + 1
							for end < len(categories) && categories[end].Section == section {
								end++
							}
							items := func() {
								for i := start; i < end; i++ {
									category := categories[i]
									if category.Placeholder != nil {
										ui.Column(c).FillWidth().MinWidth(0).Padding(t.Space(1), t.Space(2)).Children(func() { category.Placeholder(c) })
										continue
									}
									var item ui.Element
									if category.Leading != nil {
										item = SidebarItemWithLeading(c, category.Title, selected == i, category.Leading)
									} else if category.Image != nil {
										item = SidebarImageItem(c, category.Title, category.Image, selected == i)
									} else {
										item = SidebarItem(c, category.Title, category.Icon, selected == i)
									}
									if item.Key(category.ID).Clicked() {
										state.Category = category.ID
										selected = i
										state.Query = ""
									}
								}
							}
							if section != "" {
								var actions func(c *ui.Context)
								if opts.SectionActions != nil {
									actions = func(c *ui.Context) { opts.SectionActions(c, section) }
								}
								SidebarSectionWithActions(c, section, actions, items)
							} else {
								ui.Column(c).FillWidth().MinWidth(0).Gap(t.Space(1)).Children(items)
							}
							start = end
						}
					})
				})
			})
			ui.Box(c).Width(1).FillHeight().Shrink(0).Background(t.Border)
		}
		ui.Column(c).Fill().MinWidth(0).Children(func() {
			title := opts.Title
			if !opts.Narrow && !opts.HideSidebar && selected >= 0 {
				title = categories[selected].Title
			}
			if searching {
				title = opts.ResultsTitle
			}
			heading(title, true)
			selector := opts.Narrow && !opts.HideSidebar && selected >= 0
			inlineSearch := (opts.Narrow || opts.HideSidebar) && !opts.HideSearch
			if selector || inlineSearch {
				ui.Column(c).FillWidth().Padding(0, t.Space(3), t.Space(3), t.Space(3)).Gap(t.Space(2)).Children(func() {
					if selector {
						ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
							section := categories[selected].Section
							sel := Select(c, &state.Category)
							sel.Trigger.Label(opts.CategoryLabel).Grow(1).Shrink(1).MinWidth(0).Children(func() { Text(c, categories[selected].Title).SingleLine() })
							sel.Popup(func(panel ui.Element) {
								for _, category := range categories {
									if category.Placeholder != nil {
										continue
									}
									sel.Item(category.ID).Label(category.Title).Children(func() { Text(c, category.Title) })
								}
							})
							if sel.Trigger.Changed() {
								state.Query = ""
								for i := range categories {
									if categories[i].ID == state.Category {
										selected = i
									}
								}
							}
							if section != "" && opts.SectionActions != nil {
								opts.SectionActions(c, section)
							}
						})
					}
					if inlineSearch {
						search()
					}
				})
			}
			results := SearchSettings(categories, query)
			ScrollView(c, ScrollViewOptions{}).Fill().Children(func() {
				ui.Column(c).Key(state.Category).FillWidth().MinWidth(0).Padding(t.Space(5)).Gap(t.Space(5)).Children(func() {
					count, lastCategory, lastGroup := 0, -1, -1
					for _, result := range results {
						if !all && result.Category != selected {
							continue
						}
						category := categories[result.Category]
						group := category.Groups[result.Group]
						item := group.Items[result.Item]
						if lastCategory != result.Category {
							if searching || (all && len(categories) > 1) {
								Text(c, category.Title).Bold()
							}
							if category.Description != "" {
								Text(c, category.Description).TextColor(t.TextMuted).Wrap()
							}
						}
						if (lastCategory != result.Category || lastGroup != result.Group) && (group.Title != "" || group.Actions != nil) {
							ui.Column(c).FillWidth().Gap(t.Space(3)).Children(func() {
								ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
									Text(c, group.Title).Grow(1).MinWidth(0).Wrap()
									if group.Actions != nil {
										group.Actions(c)
									}
								})
								ui.Box(c).FillWidth().Height(1).Background(t.Border)
							})
						}
						lastCategory, lastGroup = result.Category, result.Group
						count++
						key := struct{ Category, Group, Item string }{category.ID, group.ID, item.ID}
						if item.Content != nil {
							ui.Column(c).Key(key).FillWidth().MinWidth(0).Children(func() { item.Content(c) })
							continue
						}
						var row ui.Element
						if opts.Narrow || item.Stacked {
							row = ui.Column(c)
						} else {
							row = ui.Row(c)
						}
						row.Key(key).FillWidth().MinWidth(0).Gap(t.Space(3)).Children(func() {
							text := func() {
								ui.Column(c).FillWidth().Shrink(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
									if item.Title != "" {
										Text(c, item.Title).Wrap()
									}
									switch {
									case item.DescriptionContent != nil:
										ui.RichText(c).FillWidth().TextColor(t.TextMuted).FontSize(t.Rem(.875)).Wrap().Children(func() { item.DescriptionContent(c) })
									case item.Description != "":
										Text(c, item.Description).TextColor(t.TextMuted).FontSize(t.Rem(.875)).Wrap()
									}
								})
							}
							if item.Leading != nil {
								ui.Row(c).FillWidth().Shrink(1).MinWidth(0).AlignItems(ui.Center).Gap(t.Space(3)).Children(func() {
									ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() { item.Leading(c) })
									text()
								})
							} else {
								text()
							}
							if item.Build != nil {
								item.Build(c)
							}
						})
					}
					if count == 0 {
						Text(c, opts.EmptyText).TextColor(t.TextMuted).Wrap()
					}
				})
			})
		})
	})
}
