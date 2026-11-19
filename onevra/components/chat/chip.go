package chat

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// ChipOptions describes a Chip's leading content and role. Leading replaces
// Image, which replaces Icon; each is optional.
type ChipOptions struct {
	Icon  func() *ui.SVG
	Image ui.ImageSource
	// Leading builds non-interactive content at the suggested size, such as
	// a product mark drawn in its own colors.
	Leading func(c *ui.Context, size float32)
	// Caret marks a chip that opens a choice.
	Caret bool
	// ReadOnly shows a fixed value, such as the current project, as an
	// undimmed, unfocusable label instead of a button.
	ReadOnly bool
}

// Chip is a compact composer or footer control for a setting such as a
// model, reasoning level, project or branch. Its label shortens with an
// ellipsis when the row is tight. Read Clicked or attach a native menu with
// Element.Menu on interactive chips; a ReadOnly chip is not a button.
func Chip(c *ui.Context, label string, options ChipOptions) ui.Element {
	t := c.Theme()
	var b ui.Element
	if options.ReadOnly {
		// Disabled dims the whole subtree in MyGo; a fixed value is text,
		// not an unavailable action.
		b = ui.Row(c).Justify(ui.Center).Shrink(0).Cursor(ui.CursorNotAllowed)
	} else {
		b = ui.ButtonBase(c)
	}
	// A coarse pointer gets the touch minimum's height; a fine one the
	// compact 26.
	b.Label(label).Height(max(float32(26), components.TouchMin(c))).MinWidth(0).Shrink(1).Padding(0, 7).Radius(6).AlignItems(ui.Center).Gap(6).
		Background(ui.Color{}).TextColor(secondary(t))
	if !options.ReadOnly && (b.Hovered() || b.Focused()) {
		b.Background(overlay(t))
	}
	if b.IsDisabled() {
		b.Opacity(.7)
	}
	b.Children(func() {
		switch {
		case options.Leading != nil:
			options.Leading(c, 14)
		case options.Image != nil:
			ui.Image(c, options.Image).Size(14, 14).Fit(ui.Contain).Radius(3).Shrink(0)
		case options.Icon != nil:
			components.Icon(c, options.Icon).Size(12, 12).Shrink(0).TextColor(tertiary(t))
		}
		components.Text(c, label).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5).TextColor(secondary(t))
		if options.Caret && !options.ReadOnly {
			components.Icon(c, lucide.ChevronDown).Size(10.5, 10.5).Shrink(0).TextColor(tertiary(t).Alpha(.7))
		}
	})
	return b
}
