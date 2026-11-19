package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const tabsIndicatorDuration = 180 * time.Millisecond

// TabsVariant selects the look of a tab list.
type TabsVariant int

const (
	// TabsUnderline marks the chosen tab with a shared accent underline that
	// animates to it. This is the default.
	TabsUnderline TabsVariant = iota
	// TabsSegmented draws the tabs as the segments of a rounded track, with
	// the chosen one raised on a shadowed, lighter thumb, as macOS draws a
	// segmented control.
	TabsSegmented
	// TabsHighlight sets the chosen tab on a softly filled, rounded block that
	// slides behind the labels, without a track.
	TabsHighlight
)

// Tabs creates a row of tabs showing labels, of which *selected is the index
// of the one chosen. A click chooses a tab, as do the arrows, Home and End
// while one has the keyboard focus, which follows the choice. Reduced Motion
// skips transitions. List's Changed reports a new choice.
//
// TabsUnderline animates a shared accent underline to the chosen tab and
// keeps every label's weight, so selection never moves the labels.
// TabsSegmented raises the chosen tab on a shadowed thumb over a capsule
// track and shows its label in a heavier weight, like a macOS segmented
// control. TabsHighlight slides a rounded block behind the chosen tab's label.
func Tabs(c *ui.Context, selected *int, variant TabsVariant, labels ...string) ui.TabsParts {
	return TabsWithLeading(c, selected, variant, nil, labels...)
}

// TabsWithLeading is Tabs with an application-built leading slot before each
// tab's label, for example an icon. The builder receives the tab's index and
// the theme-scaled icon size as a suggestion, and inherits the tab's text
// color; a nil builder leaves no leading space. Keep it non-interactive and
// free of I/O or background work.
func TabsWithLeading(c *ui.Context, selected *int, variant TabsVariant, leading func(c *ui.Context, i int, size float32), labels ...string) ui.TabsParts {
	return TabsWith(c, selected, TabsOptions{Variant: variant, Leading: leading}, labels...)
}

// TabsOptions configures TabsWith.
type TabsOptions struct {
	Variant TabsVariant
	// Leading, when set, builds the slot before each tab's label, as
	// TabsWithLeading describes.
	Leading func(c *ui.Context, i int, size float32)
	// Trailing, when set, builds the slot after each tab's label, for
	// example a count, receiving and inheriting what Leading does.
	Trailing func(c *ui.Context, i int, size float32)
	// Menu, when set, returns the builder of tab i's context menu, or nil for
	// a tab without one. The menu opens where the tab is right-clicked,
	// without choosing it, and, while it has the focus, from the menu key or
	// Shift+F10.
	Menu func(i int) func(m *ui.Menu)
}

// TabsWith is Tabs configured by options.
func TabsWith(c *ui.Context, selected *int, options TabsOptions, labels ...string) ui.TabsParts {
	tabs := ui.TabsBase(c, selected, len(labels))
	switch options.Variant {
	case TabsSegmented:
		return segmentedTabs(c, tabs, selected, labels, options)
	case TabsHighlight:
		return highlightTabs(c, tabs, selected, labels, options)
	}
	return underlinedTabs(c, tabs, selected, labels, options)
}

// tabContent builds tab i's leading and trailing slots, when given, around
// its label, and gives the tab its context menu.
func tabContent(c *ui.Context, tab ui.Element, i int, label string, options TabsOptions) ui.Element {
	t := c.Theme()
	if options.Menu != nil {
		if menu := options.Menu(i); menu != nil {
			tab.ContextMenu(menu)
		}
	}
	var text ui.Element
	tab.AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
		if options.Leading != nil {
			options.Leading(c, i, t.Space(3.5))
		}
		text = Text(c, label).SingleLine()
		if options.Trailing != nil {
			options.Trailing(c, i, t.Space(3.5))
		}
	})
	return text
}

// underlinedTabs puts the accent underline under the chosen tab.
func underlinedTabs(c *ui.Context, tabs ui.TabsParts, selected *int, labels []string, options TabsOptions) ui.TabsParts {
	t := c.Theme()
	tabs.List.Gap(t.Space(1))
	var active ui.Element
	tabs.List.Children(func() {
		for i, label := range labels {
			tab := dimmed(tabs.Tab(i).Padding(t.Space(2), t.Space(3)).FocusRing(false))
			on := i == *selected
			fg := t.Text
			if !on {
				fg = fg.Alpha(0.6)
			}
			tab.TextColor(fg)
			if on {
				active = tab
			}
			focus := tab.FocusVisible()
			tab.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if focus {
					p.FocusRing(r, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
				}
			})
			tabContent(c, tab, i, label, options)
		}
	})
	if len(labels) > 0 {
		tabs.List.DrawOver(func(p *ui.Painter, r ui.Rect) {
			// Read current layout during painting, and animate list-relative
			// geometry so moving the entire control does not move its underline.
			bounds := active.LayoutBounds()
			in, h := t.Space(1.5), t.Space(0.5)
			x := tabs.List.Animate("indicator-x", bounds.X-r.X+in, tabsIndicatorDuration)
			w := tabs.List.Animate("indicator-width", max(0, bounds.W-2*in), tabsIndicatorDuration)
			if c.Preferences().ReduceMotion {
				x, w = bounds.X-r.X+in, max(0, bounds.W-2*in)
			}
			p.Fill(ui.Rect{X: r.X + x, Y: bounds.Y + bounds.H - h, W: w, H: h}, t.Accent, h/2)
		})
	}
	return tabs
}

// segmentedTabs puts the chosen tab on a raised thumb inside a recessed track,
// as macOS draws a segmented control.
func segmentedTabs(c *ui.Context, tabs ui.TabsParts, selected *int, labels []string, options TabsOptions) ui.TabsParts {
	t := c.Theme()
	// The track is recessed from the page and the thumb raised from it, with a
	// cast shadow: the same raised face a toolbar button uses.
	track := toolbarTrack(t)
	thumb, _, _, shadow := toolbarFace(t)
	// FitRadii clamps any radius above half the height, so this makes the track
	// a capsule whatever its height and the theme's spacing.
	pill := t.Space(20)
	tabs.List.Padding(t.Space(0.5)).Gap(t.Space(0.5)).Radius(pill).Background(track).AlignSelf(ui.Start)
	var active ui.Element
	tabs.List.Children(func() {
		for i, label := range labels {
			tab := dimmed(tabs.Tab(i).Padding(t.Space(1.5), t.Space(3)).FocusRing(false))
			on := i == *selected
			fg := t.Text
			if !on {
				fg = fg.Alpha(0.6)
			}
			tab.TextColor(fg)
			if on {
				active = tab
			}
			focus := tab.FocusVisible()
			tab.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if focus {
					p.FocusRing(r, [4]float32{pill, pill, pill, pill})
				}
			})
			if text := tabContent(c, tab, i, label, options); on {
				text.FontWeight(600)
			}
		}
	})
	if len(labels) > 0 {
		// The thumb belongs behind the labels, so draw it with the track rather
		// than over the tabs.
		tabs.List.Draw(func(p *ui.Painter, r ui.Rect) {
			thumbRect := slideBehind(c, tabs.List, active, r)
			radius := thumbRect.H / 2
			p.Shadow(thumbRect, radius, 0, 1, 2, 0, shadow)
			p.Fill(thumbRect, thumb, radius)
		})
	}
	return tabs
}

// highlightTabs sets the chosen tab on a filled block that slides behind the
// labels, as Figma marks its chosen tab.
func highlightTabs(c *ui.Context, tabs ui.TabsParts, selected *int, labels []string, options TabsOptions) ui.TabsParts {
	t := c.Theme()
	radius := [4]float32{t.Radius, t.Radius, t.Radius, t.Radius}
	tabs.List.Gap(t.Space(0.5)).AlignSelf(ui.Start)
	var active ui.Element
	tabs.List.Children(func() {
		for i, label := range labels {
			tab := dimmed(tabs.Tab(i).Padding(t.Space(1.5), t.Space(3)).FocusRing(false))
			on := i == *selected
			fg := t.Text
			if !on {
				fg = fg.Alpha(0.6)
			}
			tab.TextColor(fg)
			if on {
				active = tab
			}
			focus := tab.FocusVisible()
			tab.DrawOver(func(p *ui.Painter, r ui.Rect) {
				if focus {
					p.FocusRing(r, radius)
				}
			})
			tabContent(c, tab, i, label, options)
		}
	})
	if len(labels) > 0 {
		tabs.List.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(slideBehind(c, tabs.List, active, r), t.Text.Alpha(0.08), t.Radius)
		})
	}
	return tabs
}

// slideBehind returns the rectangle of a block that slides to the chosen tab
// inside list's rectangle r. It animates list-relative geometry, so moving the
// entire control does not move the block, and Reduced Motion skips the slide.
func slideBehind(c *ui.Context, list, active ui.Element, r ui.Rect) ui.Rect {
	bounds := active.LayoutBounds()
	x := list.Animate("thumb-x", bounds.X-r.X, tabsIndicatorDuration)
	w := list.Animate("thumb-width", bounds.W, tabsIndicatorDuration)
	if c.Preferences().ReduceMotion {
		x, w = bounds.X-r.X, bounds.W
	}
	return ui.Rect{X: r.X + x, Y: bounds.Y, W: w, H: bounds.H}
}
