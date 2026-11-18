package components

import "github.com/egoist/mygo/ui"

// Sidebar creates a transparent column of SidebarSections. Place it inside a
// ui.Scroll when scrolling is needed. Applications own its width, titlebar
// inset and selection; it can be used independently of Shell.
func Sidebar(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).FillWidth().MinWidth(0).
		Padding(0, t.Space(3), t.Space(3), t.Space(3)).Gap(t.Space(3))
}

// SidebarSection builds a muted heading followed by application-owned items.
// Sections are always expanded; fn may be nil for a heading alone.
func SidebarSection(c *ui.Context, label string, fn func()) ui.Element {
	return SidebarSectionWithActions(c, label, nil, fn)
}

// SidebarSectionWithActions is a SidebarSection whose heading ends in
// application-built controls, such as an add button. IconButtonCompact fits
// the heading without raising it. A nil actions builder leaves the heading
// as SidebarSection's. Like the items, actions run on the UI thread and must
// not start I/O or background work.
func SidebarSectionWithActions(c *ui.Context, label string, actions func(c *ui.Context), fn func()) ui.Element {
	t := c.Theme()
	section := ui.Column(c).FillWidth().MinWidth(0).Gap(t.Space(1))
	section.Children(func() {
		heading := func() ui.Element {
			return Text(c, label).FontSize(t.Rem(.8)).Bold().TextColor(t.TextMuted).
				Padding(t.Space(1), t.Space(2), t.Space(1), t.Space(2))
		}
		if actions == nil {
			heading()
		} else {
			ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				heading().Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
				actions(c)
			})
		}
		if fn != nil {
			fn()
		}
	})
	return section
}

// SidebarItem creates a navigation button with a neutral selected face and an
// optional accent-colored icon. Hover does not change its face. The caller
// supplies selection and handles Clicked; MyGo provides Tab focus and activation
// with Enter or Space, not tree-style arrow navigation. Key can distinguish
// items with identical labels. A nil icon leaves no icon space.
func SidebarItem(c *ui.Context, label string, icon *ui.SVG, selected bool) ui.Element {
	return SidebarItemWithLeading(c, label, selected, func(c *ui.Context, size float32) {
		if icon != nil {
			Icon(c, icon).Size(size, size).TextColor(c.Theme().Accent)
		}
	})
}

// SidebarImageItem is a SidebarItem led by an image, such as an application's
// project logo, in place of the icon. The image keeps its own colors, fits
// within the icon's square without cropping and has slightly rounded corners.
// A nil image leaves no icon space.
func SidebarImageItem(c *ui.Context, label string, image ui.ImageSource, selected bool) ui.Element {
	return SidebarItemWithLeading(c, label, selected, func(c *ui.Context, size float32) {
		if image != nil {
			ui.Image(c, image).Size(size, size).Fit(ui.Contain).Radius(size * .2).Shrink(0)
		}
	})
}

// SidebarItemWithLeading is a SidebarItem with an application-built leading
// slot, for example an image, monogram or status mark. A nil builder leaves no
// leading space. The builder receives the theme-scaled standard icon size as a
// suggestion and owns its content's size and appearance. Keep it non-interactive
// so the navigation button remains the only focus and activation target.
// It runs synchronously on the UI thread and must not start I/O or background work.
func SidebarItemWithLeading(c *ui.Context, label string, selected bool, leading func(c *ui.Context, size float32)) ui.Element {
	return SidebarItemWithSlots(c, label, selected, leading, nil)
}

// SidebarItemWithSlots is a SidebarItemWithLeading with an application-built
// trailing slot after the label, for example counts or a status mark. The
// label then takes the remaining room and ends in an ellipsis when it does
// not fit. Nil builders leave no space. Both builders receive the standard
// icon size and follow the leading slot's rules: keep them non-interactive
// and free of I/O or background work.
func SidebarItemWithSlots(c *ui.Context, label string, selected bool, leading, trailing func(c *ui.Context, size float32)) ui.Element {
	return SidebarItemWithContent(c, label, selected, leading, nil, trailing)
}

// SidebarItemWithContent is a SidebarItemWithSlots whose visible label is
// application-built, for example animated or styled text; label still keys
// the item and names it for accessibility. A nil content builder shows label
// as plain text. With a trailing slot, the returned element takes the
// remaining room and may shrink below its natural width. Content follows the
// leading slot's rules.
func SidebarItemWithContent(c *ui.Context, label string, selected bool, leading func(c *ui.Context, size float32), content func(c *ui.Context) ui.Element, trailing func(c *ui.Context, size float32)) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Key(label).Label(label).FillWidth().Justify(ui.Start).
		Padding(t.Space(1.75), t.Space(2)).Gap(t.Space(2)).Radius(t.Radius).
		Selected(selected).Background(ui.Color{}).TextColor(t.Text)
	if selected {
		b.Background(t.Text.Alpha(.1))
	}
	b.Children(func() {
		if leading != nil {
			leading(c, t.Space(4.5))
		}
		var text ui.Element
		if content != nil {
			text = content(c)
		} else {
			text = Text(c, label).SingleLine()
		}
		if trailing != nil {
			text.Grow(1).Shrink(1).MinWidth(0).Ellipsis("…")
			trailing(c, t.Space(4.5))
		}
	})
	return dimmed(b)
}
