package chat

import (
	"strings"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// Option is one Picker choice. ID is stable and nonempty. Group gathers
// consecutive options under a heading; Detail is trailing secondary text,
// such as a context size, and Description a muted second line. Leading
// replaces Image, which replaces Icon; when any option has one, the others
// keep its space.
type Option struct {
	ID, Label, Detail, Description, Group string
	Icon                                  func() *ui.SVG
	Image                                 ui.ImageSource
	Leading                               func(c *ui.Context, size float32)
}

// PickerState retains whether the popover is open and its search text.
type PickerState struct {
	Open  bool
	Query string
}

// PickerOptions describes a chip-triggered choice, such as a model, reasoning
// level or branch. Label is the trigger text and accessible name; Icon,
// Image and Leading lead it as in ChipOptions. SearchLabel enables filtering
// once there are more than Searchable options (default 8); EmptyText shows
// when nothing matches. Width defaults to 260.
type PickerOptions struct {
	Label, SearchLabel, EmptyText string
	Icon                          func() *ui.SVG
	Image                         ui.ImageSource
	Leading                       func(c *ui.Context, size float32)
	Options                       []Option
	Selected                      string
	Width                         float32
	Searchable                    int
}

// PickerResult reports the trigger and a frame-local choice.
type PickerResult struct {
	Trigger ui.Element
	Chosen  string
}

// Picker shows a Chip that opens a popover of grouped, optionally searchable
// options with a check on the selection. Choosing closes the popover; the
// application applies and persists the choice.
func Picker(c *ui.Context, state *PickerState, options PickerOptions) PickerResult {
	t := c.Theme()
	trigger := Chip(c, options.Label, ChipOptions{Icon: options.Icon, Image: options.Image, Leading: options.Leading, Caret: true})
	if state.Open {
		trigger.Background(overlayStrong(t))
	}
	if trigger.Clicked() {
		state.Open, state.Query = !state.Open, ""
	}
	var chosen string
	width := options.Width
	if width <= 0 {
		width = 260
	}
	threshold := options.Searchable
	if threshold <= 0 {
		threshold = 8
	}
	components.Popover(c, trigger, &state.Open, func(panel ui.Element) {
		panel.Label(options.Label).Width(width).Padding(0).Gap(0).Children(func() {
			if options.SearchLabel != "" && len(options.Options) > threshold {
				ui.Row(c).FillWidth().Padding(8, 8, 0, 8).Children(func() {
					components.TextBox(c, &state.Query, components.TextBoxOptions{LeadingIcon: lucide.Search()}).
						FillWidth().AutoFocus().Label(options.SearchLabel).Placeholder(options.SearchLabel)
				})
			}
			query := strings.ToLower(strings.TrimSpace(state.Query))
			var visible []Option
			leading := false
			for _, o := range options.Options {
				leading = leading || o.Leading != nil || o.Image != nil || o.Icon != nil
				if query == "" || strings.Contains(strings.ToLower(o.Label+" "+o.Group+" "+o.Detail+" "+o.ID), query) {
					visible = append(visible, o)
				}
			}
			if len(visible) == 0 {
				components.Text(c, options.EmptyText).FontSize(12.5).TextColor(tertiary(t)).Padding(12)
				return
			}
			// The list scrolls edge to edge; its padding insets the rows.
			components.ScrollView(c, components.ScrollViewOptions{}).FillWidth().MaxHeight(348).Padding(4).Gap(1).Children(func() {
				group := ""
				for i, o := range visible {
					if o.Group != "" && (i == 0 || o.Group != group) {
						components.Text(c, o.Group).FontSize(11.5).FontWeight(500).TextColor(tertiary(t)).Padding(8, 8, 3, 8)
					}
					group = o.Group
					if pickerRow(c, o, o.ID == options.Selected, leading) {
						chosen = o.ID
						state.Open = false
					}
				}
			})
		})
	})
	return PickerResult{Trigger: trigger, Chosen: chosen}
}

func pickerRow(c *ui.Context, o Option, selected, leading bool) bool {
	t := c.Theme()
	b := ui.ButtonBase(c).Key(o.ID).Label(o.Label).Selected(selected).FillWidth().MinWidth(0).Padding(6, 8).Radius(6).
		Justify(ui.Start).AlignItems(ui.Center).Gap(8).Background(ui.Color{}).TextColor(t.Text)
	if b.Hovered() || b.Focused() {
		b.Background(overlay(t))
	}
	b.Children(func() {
		switch {
		case o.Leading != nil:
			o.Leading(c, 16)
		case o.Image != nil:
			ui.Image(c, o.Image).Size(16, 16).Fit(ui.Contain).Radius(3).Shrink(0)
		case o.Icon != nil:
			ui.Row(c).Size(16, 16).Shrink(0).Center().Children(func() {
				components.Icon(c, o.Icon).Size(13, 13).TextColor(secondary(t))
			})
		case leading:
			ui.Box(c).Size(16, 16).Shrink(0)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			components.Text(c, o.Label).MinWidth(0).SingleLine().Ellipsis("…").FontSize(13)
			if o.Description != "" {
				components.Text(c, o.Description).MinWidth(0).Wrap().FontSize(11.5).TextColor(tertiary(t))
			}
		})
		if o.Detail != "" {
			components.Text(c, o.Detail).Shrink(0).FontSize(11.5).TextColor(tertiary(t))
		}
		mark := ui.Box(c).Size(12, 12).Shrink(0)
		if selected {
			mark.Children(func() { components.Icon(c, lucide.Check).Size(12, 12).TextColor(t.Accent) })
		}
	})
	return b.Clicked()
}
