// Package command provides application-owned command views and dialogs.
// CommandView embeds search and results in any bounded parent, such as a
// panel window; CommandDialog hosts the same view in Onevra's modal surface.
// Hosts register opening shortcuts, retain State, and execute activated IDs.
package command

import (
	"slices"
	"strings"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

// Item describes a command. IDs must be nonempty and unique, independent of
// localized titles. Shortcut is display text; the host registers actual keys.
// Items in the same Group are displayed together in first-appearance order.
type Item struct {
	ID, Title, Description, Group, Shortcut string
	Keywords                                []string
	Icon                                    *ui.SVG
	// Leading replaces Icon with non-interactive content of the given size,
	// such as an image. Trailing adds non-interactive content before Shortcut.
	// Neither slot takes part in filtering.
	Leading  func(c *ui.Context, size float32)
	Trailing func(c *ui.Context)
	Disabled bool
}

// State is retained by the host for one command view; do not copy it after use.
// Query and ActiveID may be updated by the host. When the query or results
// change, the first available command becomes active and the results scroll to
// the top, unless the host also set ActiveID. The zero value is ready to use.
// Closing a dialog preserves the query; hosts choose when to reset it.
type State struct {
	// InputSourceError records the last selection or restoration failure.
	// A failure never prevents searching. It is cleared on the next attempt.
	InputSourceError      error
	inputSource           inputSourceSession
	Query                 string
	ActiveID              string
	scroll                components.ListState
	lastQuery, lastActive string
	lastIDs               []string
	lastIndex             int
	composing             bool
	refocus               bool
}

// Options supplies application text, results and activation behavior.
type Options struct {
	// InputSource optionally selects an enabled macOS input source once when
	// search receives focus, e.g. InputSourceABC. Empty keeps the current source.
	// Selection waits for any marked-text composition to finish. Unsupported
	// platforms and headless views report State.InputSourceError without switching.
	InputSource string
	// KeepInputSource skips restoration on close. By default restoration only
	// occurs if the user has not manually selected a different source.
	KeepInputSource bool
	// ExpandOnSearch hides the results area until Query contains non-whitespace
	// committed text, and collapses it again when cleared. Hidden results cannot
	// be activated; loading, error and empty states are also hidden until search.
	// The zero value shows the results area immediately.
	ExpandOnSearch bool
	// Filter replaces the package Filter for synchronous local matching. It
	// receives this frame's query and must not modify items. Nil uses Filter.
	Filter func(items []Item, query string) []Item
	// PreFiltered uses host-supplied matches instead of applying a filter.
	// For asynchronous search, the host owns cancellation and stale-result checks.
	PreFiltered bool
	// Loading and Error replace results and suppress activation. Error wins.
	Loading bool
	Error   string
	// Empty labels use Onevra's catalog in the window's language.
	SearchLabel, Placeholder, ResultsLabel, EmptyText, LoadingText string
	// OnActivate runs on the UI thread for Enter or a single click. The host
	// owns execution, background work and whether to close the dialog or window.
	OnActivate func(id string)
}

// Filter returns matching items without modifying the input. All whitespace-
// separated query terms must match the title or keywords, case-insensitively.
// Exact titles rank before title prefixes, title substrings, then keywords;
// ties retain input order. An empty query returns a copy in original order.
func Filter(items []Item, query string) []Item {
	terms := strings.Fields(strings.ToLower(query))
	type match struct {
		item  Item
		score int
	}
	matches := make([]match, 0, len(items))
	for _, item := range items {
		title := strings.ToLower(item.Title)
		keywords := strings.ToLower(strings.Join(item.Keywords, " "))
		score, ok := 0, true
		for _, term := range terms {
			switch {
			case title == term:
			case strings.HasPrefix(title, term):
				score += 1
			case strings.Contains(title, term):
				score += 2
			case strings.Contains(keywords, term):
				score += 3
			default:
				ok = false
			}
		}
		if ok {
			matches = append(matches, match{item, score})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int { return a.score - b.score })
	out := make([]Item, len(matches))
	for i, m := range matches {
		out[i] = m.item
	}
	return out
}

func grouped(items []Item) []Item {
	var groups []string
	buckets := make(map[string][]Item)
	for _, item := range items {
		if _, ok := buckets[item.Group]; !ok {
			groups = append(groups, item.Group)
		}
		buckets[item.Group] = append(buckets[item.Group], item)
	}
	out := make([]Item, 0, len(items))
	for _, group := range groups {
		out = append(out, buckets[group]...)
	}
	return out
}

// Focus moves keyboard focus to the search field when the view is next built,
// for example after the host shows a hidden window. CommandDialog focuses
// search on opening without it.
func (s *State) Focus() { s.refocus = true }

// sync reconciles ActiveID with this frame's results. When the query or the
// results change, the first available command becomes active and the list
// returns to the top, unless the host chose ActiveID itself since last frame.
func (s *State) sync(items []Item) int {
	changed := s.Query != s.lastQuery || len(items) != len(s.lastIDs)
	for i := 0; !changed && i < len(items); i++ {
		changed = items[i].ID != s.lastIDs[i]
	}
	top := changed && s.ActiveID == s.lastActive
	if top {
		s.ActiveID = ""
	}
	index := -1
	for i, item := range items {
		if !item.Disabled && item.ID != "" && (index < 0 || item.ID == s.ActiveID) {
			index = i
			if item.ID == s.ActiveID {
				break
			}
		}
	}
	s.ActiveID = ""
	if index >= 0 {
		s.ActiveID = items[index].ID
	}
	switch {
	case top && index >= 0:
		s.scroll.ScrollTo(0, ui.Start)
	case changed || s.ActiveID != s.lastActive || index != s.lastIndex:
		if index >= 0 {
			s.scroll.ScrollIntoView(index)
		}
	}
	if changed {
		s.lastIDs = s.lastIDs[:0]
		for _, item := range items {
			s.lastIDs = append(s.lastIDs, item.ID)
		}
	}
	s.lastQuery, s.lastActive = s.Query, s.ActiveID
	s.lastIndex = index
	return index
}

func (s *State) move(items []Item, delta int) {
	i := s.sync(items)
	for next := i + delta; next >= 0 && next < len(items); next += delta {
		if !items[next].Disabled && items[next].ID != "" {
			s.ActiveID = items[next].ID
			s.scroll.ScrollIntoView(next)
			return
		}
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func resultsExpanded(s *State, options Options) bool {
	return !options.ExpandOnSearch || strings.TrimSpace(s.Query) != ""
}
