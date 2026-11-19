package command

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// CommandDialog hosts CommandView in Onevra's modal surface. Call every
// frame with retained open and state. The host owns opening shortcuts, query
// resets and closing on activation. Dialog dismissal and focus restoration
// follow components.Dialog. Its returned panel may be resized in this frame.
// With ExpandOnSearch, its height fits the input until a query is committed.
func CommandDialog(c *ui.Context, open *bool, s *State, items []Item, options Options, dialog components.DialogOptions) ui.Element {
	if !*open {
		_ = s.ReleaseInputSource() // Restoration failures remain available in s.InputSourceError.
		s.composing = false
		s.refocus = false
	}
	// Dismissal is evaluated before the input is built. Preserve the previous
	// frame's marked-text state so Escape ending composition cannot also close
	// the surrounding dialog. Text composition remains owned by MyGo.
	dialog.DisableEscapeClose = dialog.DisableEscapeClose || s.composing
	if dialog.Label == "" {
		dialog.Label = fallback(options.SearchLabel, l10n.In(c.Locale()).Text("commands"))
	}
	panel := components.Dialog(c, open, dialog, func(panel ui.Element) {
		panel.Width(640).Padding(0).Gap(0).Radius(c.Theme().Radius * 3).Clip()
		CommandView(c, s, items, options)
		// Read after the view resolves bound text input so expansion and
		// collapse use this frame's query, without rebuilding the search field.
		if resultsExpanded(s, options) {
			panel.Height(440)
		}
	})
	if !*open {
		_ = s.ReleaseInputSource() // Restoration failures remain available in s.InputSourceError.
	}
	return panel
}
