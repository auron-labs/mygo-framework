package components

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ScrollEdgeStyle selects what separates a ScrollEdgeBar from the content
// scrolling under it.
type ScrollEdgeStyle int

const (
	// ScrollEdgeGlass sets the whole bar on one capsule of Liquid Glass and
	// leaves the content beside it untouched. This is the default.
	ScrollEdgeGlass ScrollEdgeStyle = iota
	// ScrollEdgeSoft fades the content into the background toward the edge,
	// without a blur, as macOS's soft scroll edge effect.
	ScrollEdgeSoft
	// ScrollEdgeHard frosts the content evenly under the bar and draws a
	// hairline where it ends, as macOS's hard scroll edge effect.
	ScrollEdgeHard
	// ScrollEdgeBlur blurs the content progressively, the more the nearer
	// the edge.
	ScrollEdgeBlur
)

// ScrollEdgeBarOptions configures a ScrollEdgeBar.
type ScrollEdgeBarOptions struct {
	Style ScrollEdgeStyle
	// Bottom docks the bar at the bottom edge, with its effect rising from
	// it, rather than at the top.
	Bottom bool
	// Height is the bar's height in DIPs; zero is 52, which fits 40-DIP
	// buttons.
	Height float32
	// Inset is the bar's distance from its edge and from the sides in DIPs;
	// zero is three theme spacing units.
	Inset float32
	// Glass is the bar's material in ScrollEdgeGlass, such as glass.Clear
	// or a tint; Interactive is ignored. The zero value is regular glass.
	Glass glass.Glass
	// Color makes the bar less see-through: it lays over the glass in
	// ScrollEdgeGlass, over the blur in ScrollEdgeBlur, fading out with it,
	// and is the bar's own surface in ScrollEdgeSoft and ScrollEdgeHard.
	// Unset, glass takes Background at 85% and blur at 93%; soft and hard
	// bars stay transparent.
	Color ui.Color
	// Uncolored leaves glass and blur bare, without Color.
	Uncolored bool
	// Background is what the content scrolls over, which ScrollEdgeSoft and
	// ScrollEdgeHard fade toward; unset, the theme's Background.
	Background ui.Color
}

// Extra room the effects take beyond the bar, as MyGo's glass gallery and
// macOS size them: a soft edge fades out 10 DIPs past the bar, a
// progressive blur 24 DIPs past it.
const (
	scrollEdgeSoftExtra = 10
	scrollEdgeBlurExtra = 24
	scrollEdgeBlur      = 6
)

// The alphas of the default color of glass and blur, and of the background
// at the bar's edge in every style but glass.
const (
	scrollEdgeGlassColor = 0.85
	scrollEdgeBlurColor  = 0.93
	scrollEdgeBackground = 0.93
)

// The alphas at which MyGo's glass.ScrollEdge lays the background over the
// content at the bar's edge, soft and hard.
const (
	glassSoftEdge = 0.85
	glassHardEdge = 0.8245
)

func (o ScrollEdgeBarOptions) resolved(c *ui.Context) ScrollEdgeBarOptions {
	if o.Height <= 0 {
		o.Height = 52
	}
	if o.Inset <= 0 {
		o.Inset = c.Theme().Space(3)
	}
	if o.Uncolored {
		o.Color = ui.Color{}
	} else if (o.Style == ScrollEdgeGlass || o.Style == ScrollEdgeBlur) && o.Color.A == 0 {
		o.Color = o.Background
		if o.Color.A == 0 {
			o.Color = c.Theme().Background
		}
		if o.Style == ScrollEdgeGlass {
			o.Color = o.Color.Alpha(scrollEdgeGlassColor)
		} else {
			o.Color = o.Color.Alpha(scrollEdgeBlurColor)
		}
	}
	return o
}

// ScrollEdgeBarExtent returns how far from its edge a ScrollEdgeBar with
// options reaches, in DIPs: the scroll content's padding on that edge, plus
// any spacing, and its ScrollbarInsets keep content and scroll bar clear of
// the bar.
func ScrollEdgeBarExtent(c *ui.Context, options ScrollEdgeBarOptions) float32 {
	o := options.resolved(c)
	return o.Inset + o.Height
}

// ScrollEdgeBar floats a bar of controls over the top or bottom edge of
// scrolling content, such as a title bar, toolbar or bottom bar, and
// separates it from the content in the chosen style. Build it after the
// content, as a child of a bounded, clipping container that the content
// fills: materials sample what was painted beneath them, and the bar and
// its effect position themselves absolutely within that container.
//
//	ui.Box(c).Fill().Clip().Children(func() {
//		inset := components.ScrollEdgeBarExtent(c, options)
//		components.ScrollView(c, components.ScrollViewOptions{}).Fill().
//			Padding(inset+12, 16, 16).ScrollbarInsets(inset, 0, 0).Children(content)
//		components.ScrollEdgeBar(c, options, toolbar).Label("Toolbar")
//	})
//
// The returned element is the bar, a centered row that content builds the
// children of; the application owns them, their actions and the bar's
// accessible label. In styles other than ScrollEdgeGlass, the bar is
// transparent, so the controls should carry their own surfaces, such as
// interactive glass. The effect layer never takes input, so content under
// it beside the bar stays reachable.
func ScrollEdgeBar(c *ui.Context, options ScrollEdgeBarOptions, content func()) ui.Element {
	o := options.resolved(c)
	edge := ScrollEdgeOptions{Style: o.Style, Bottom: o.Bottom, Extent: o.Inset + o.Height, Background: o.Background}
	if o.Style == ScrollEdgeBlur {
		edge.Color, edge.Uncolored = o.Color, o.Uncolored
	}
	ScrollEdge(c, edge)
	bar := ui.Row(c).Absolute().Left(o.Inset).Right(o.Inset).Height(o.Height).
		Padding(0, 6).Gap(6).AlignItems(ui.Center).Radius(o.Height / 2)
	if o.Bottom {
		bar.Bottom(o.Inset)
	} else {
		bar.Top(o.Inset)
	}
	if o.Style == ScrollEdgeGlass {
		material := o.Glass
		material.Interactive = false
		if o.Color.A > 0 {
			bar.Material(tinted{material, ui.LinearGradient{From: o.Color, To: o.Color}})
		} else {
			bar.Material(material)
		}
	} else if o.Style != ScrollEdgeBlur && o.Color.A > 0 {
		bar.Background(o.Color)
	} else {
		bar.PassThrough()
	}
	if content != nil {
		bar.Children(content)
	}
	return bar
}

// tinted paints a color, solid or fading, over a material.
type tinted struct {
	material ui.Material
	tint     ui.LinearGradient
}

func (m tinted) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	m.material.PaintMaterial(p, box, radii)
	p.FillGradient(box, m.tint, radii[0])
}

func (m tinted) String() string {
	c := m.tint.From
	return fmt.Sprintf("rgba(%d, %d, %d, %d) over %v", c.R, c.G, c.B, c.A, m.material)
}

// ScrollEdgeOptions configures a ScrollEdge.
type ScrollEdgeOptions struct {
	Style  ScrollEdgeStyle
	Bottom bool
	// Extent is how far from its edge the effect reaches in DIPs, such as
	// the height of the controls floating there; soft edges and progressive
	// blur fade out a little beyond it.
	Extent float32
	// Background is what the content scrolls over, which ScrollEdgeSoft and
	// ScrollEdgeHard fade toward; unset, the theme's Background.
	Background ui.Color
	// Color tints ScrollEdgeBlur's blur, fading out with it, so that
	// controls over it stay legible; unset, Background at 93%.
	Color ui.Color
	// Uncolored leaves the blur bare, without Color.
	Uncolored bool
}

// ScrollEdge is ScrollEdgeBar's effect alone, for hosts that float their own
// controls over an edge of scrolling content, such as a composer. Build it
// after the content in the same bounded, clipping container. It never takes
// input; ScrollEdgeGlass has no edge of its own, so it draws nothing.
func ScrollEdge(c *ui.Context, options ScrollEdgeOptions) ui.Element {
	edge := ui.Box(c).Absolute().Left(0).Right(0).PassThrough()
	if options.Bottom {
		edge.Bottom(0)
	} else {
		edge.Top(0)
	}
	extent := max(0, options.Extent)
	// More of the background over MyGo's soft and hard edges, so that
	// together they reach scrollEdgeBackground at the bar's edge.
	bg := options.Background
	if bg.A == 0 {
		bg = c.Theme().Background
	}
	thicken := func(base float32) ui.Color {
		return bg.Alpha(1 - (1-scrollEdgeBackground)/(1-base))
	}
	switch options.Style {
	case ScrollEdgeGlass:
		edge.Height(extent)
	case ScrollEdgeHard:
		over := thicken(glassHardEdge)
		material := glass.ScrollEdge{Hard: true, Bottom: options.Bottom, Background: options.Background}
		edge.Height(extent).Material(tinted{material, ui.LinearGradient{From: over, To: over}})
	case ScrollEdgeBlur:
		mask := &ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180, Start: 0.3, End: 1}
		if options.Bottom {
			mask.Angle = 0
		}
		blur := glass.Blur{Radius: scrollEdgeBlur, Mask: mask}
		edge.Height(extent + scrollEdgeBlurExtra)
		color := options.Color
		if color.A == 0 {
			color = bg.Alpha(scrollEdgeBlurColor)
		}
		if !options.Uncolored {
			tint := *mask
			tint.From, tint.To = color, color.Alpha(0)
			edge.Material(tinted{blur, tint})
		} else {
			edge.Material(blur)
		}
	default:
		over := ui.LinearGradient{From: thicken(glassSoftEdge), To: bg.Alpha(0), Angle: 180}
		if options.Bottom {
			over.Angle = 0
		}
		material := glass.ScrollEdge{Bottom: options.Bottom, Background: options.Background}
		edge.Height(extent + scrollEdgeSoftExtra).Material(tinted{material, over})
	}
	return edge
}
