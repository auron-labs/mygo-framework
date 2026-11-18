package components

import "github.com/egoist/mygo/ui"

// ScrollAxes selects the directions in which a ScrollView can scroll.
type ScrollAxes uint8

const (
	ScrollVertical ScrollAxes = iota
	ScrollHorizontal
	ScrollBoth
)

// ScrollViewOptions configures a ScrollView.
type ScrollViewOptions struct {
	// Axes defaults to ScrollVertical, including for unrecognized values.
	Axes ScrollAxes
}

// ScrollView creates a scroll container using MyGo's layout, clipping, input
// and scroll state. Give it a size or Fill/Grow within a bounded parent.
// Vertical and both-axis views arrange children in a column; horizontal views
// arrange them in a row. Column/Row on the returned element can override this
// arrangement without changing the scrolling axes.
//
// ScrollbarInsets on the returned element sets top, right, bottom and left
// track insets in DIPs (CSS shorthand), independently of content padding:
//
//	ScrollView(c, ScrollViewOptions{}).Fill().ScrollbarInsets(48, 0, 16)
//
// TrackScroll accepts application-owned ui.ScrollState. MyGo owns scrollbar
// appearance and visibility: overflowing axes show bars on hover or while
// dragging, with color and width from the frame theme. Independent visibility
// and per-view color/width overrides are not supported by the upstream API.
func ScrollView(c *ui.Context, options ScrollViewOptions) ui.Element {
	switch options.Axes {
	case ScrollHorizontal:
		return ui.ScrollHorizontal(c)
	case ScrollBoth:
		return ui.ScrollBoth(c)
	default:
		return ui.Scroll(c)
	}
}
