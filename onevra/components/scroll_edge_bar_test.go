package components

import (
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// The bar docks at its edge, its effect stays out of input beside it, and each
// style paints what is under the edge as it should, flipped at the bottom.
func TestScrollEdgeBar(t *testing.T) {
	for _, bottom := range []bool{false, true} {
		for _, style := range []ScrollEdgeStyle{ScrollEdgeGlass, ScrollEdgeSoft, ScrollEdgeHard, ScrollEdgeBlur} {
			t.Run(fmt.Sprintf("%v/%d", bottom, style), func(t *testing.T) {
				clicked, actions := 0, 0
				var extent float32
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(false))
					options := ScrollEdgeBarOptions{Style: style, Bottom: bottom}
					extent = ScrollEdgeBarExtent(c, options)
					ui.Box(c).Fill().Clip().Children(func() {
						ui.Box(c).Fill().Background(ui.RGB(0, 0, 0))
						under := Button(c, "Under", ButtonSecondary).Absolute().Left(0).Width(4).Height(4)
						if bottom {
							under.Bottom(0)
						} else {
							under.Top(0)
						}
						if under.Clicked() {
							clicked++
						}
						ScrollEdgeBar(c, options, func() {
							if Button(c, "Action", ButtonSecondary).Clicked() {
								actions++
							}
						}).Label("Bar")
					})
				}, 300, 300)
				bar, ok := tt.Find("Bar")
				if !ok || extent != 64 || bar.H != 52 || bar.X != 12 || bar.W != 276 {
					t.Fatalf("bar %v, extent %v", bar, extent)
				}
				if (!bottom && bar.Y != 12) || (bottom && bar.Y+bar.H != 288) {
					t.Fatalf("bar %v not docked at its edge", bar)
				}
				if err := tt.Click("Action"); err != nil || actions != 1 {
					t.Fatalf("bar action: %v, %d", err, actions)
				}
				if err := tt.Click("Under"); err != nil || clicked != 1 {
					t.Fatalf("content beside the bar: %v, %d", err, clicked)
				}
				// The edge's row above or below the bar, and the far middle.
				img := tt.Image()
				s := float32(img.Bounds().Dx()) / 300
				near, far := 6, 150
				if bottom {
					near, far = 293, 150
				}
				at := func(y int) uint8 { return img.RGBAAt(int(150*s), int(float32(y)*s)).R }
				edge, middle := at(near), at(far)
				if middle > 8 {
					t.Fatalf("content away from the bar changed: %d", middle)
				}
				switch style {
				case ScrollEdgeGlass:
					// Uniform black stays black outside the bar.
					if edge > 8 {
						t.Fatalf("content outside the bar changed: %d", edge)
					}
				default:
					// Soft, hard and tinted blurred edges lay the light
					// background over it, at about 93% next to the bar.
					if edge < 218 {
						t.Fatalf("edge not toward the background: %d", edge)
					}
				}
			})
		}
	}
}

// An effect alone docks at its edge and reaches its extent plus the style's
// fade, beside nothing it takes input from.
func TestScrollEdge(t *testing.T) {
	for _, tc := range []struct {
		style  ScrollEdgeStyle
		bottom bool
		height float32
	}{{ScrollEdgeGlass, false, 40}, {ScrollEdgeHard, true, 40}, {ScrollEdgeSoft, false, 50}, {ScrollEdgeBlur, true, 64}} {
		clicked := 0
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Clip().Children(func() {
				if Button(c, "Under", ButtonSecondary).Fill().Clicked() {
					clicked++
				}
				ScrollEdge(c, ScrollEdgeOptions{Style: tc.style, Bottom: tc.bottom, Extent: 40}).Label("Edge")
			})
		}, 300, 300)
		edge, ok := tt.Find("Edge")
		if !ok || edge.H != tc.height || edge.W != 300 || (tc.bottom && edge.Y+edge.H != 300) || (!tc.bottom && edge.Y != 0) {
			t.Fatalf("style %d: edge %v", tc.style, edge)
		}
		tt.ClickAt(150, edge.Y+edge.H/2)
		if clicked != 1 {
			t.Fatalf("style %d: edge took input", tc.style)
		}
	}
}

// A blurred edge alone takes the background over it by default, so text
// over it stays legible, and stays bare uncolored.
func TestScrollEdgeBlurColor(t *testing.T) {
	for _, tc := range []struct {
		options ScrollEdgeOptions
		r       [2]uint8 // the edge's red bounds over black
	}{
		{ScrollEdgeOptions{Style: ScrollEdgeBlur, Bottom: true, Extent: 40}, [2]uint8{218, 255}},
		{ScrollEdgeOptions{Style: ScrollEdgeBlur, Bottom: true, Extent: 40, Uncolored: true}, [2]uint8{0, 16}},
	} {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(false))
			ui.Box(c).Fill().Clip().Children(func() {
				ui.Box(c).Fill().Background(ui.RGB(0, 0, 0))
				ScrollEdge(c, tc.options)
			})
		}, 300, 300)
		img := tt.Image()
		s := float32(img.Bounds().Dx()) / 300
		if px := img.RGBAAt(int(150*s), int(294*s)); px.R < tc.r[0] || px.R > tc.r[1] {
			t.Fatalf("%+v: edge %v", tc.options, px)
		}
	}
}

// A color lays over the bar in every style, under its controls; glass and
// blur take the background by default unless uncolored.
func TestScrollEdgeBarColor(t *testing.T) {
	red := ui.RGB(255, 0, 0)
	for _, tc := range []struct {
		options ScrollEdgeBarOptions
		r, g    [2]uint8 // the bar's red and green bounds over black
	}{
		{ScrollEdgeBarOptions{Style: ScrollEdgeGlass, Color: red}, [2]uint8{240, 255}, [2]uint8{0, 16}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeSoft, Color: red}, [2]uint8{240, 255}, [2]uint8{0, 16}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeHard, Color: red}, [2]uint8{240, 255}, [2]uint8{0, 16}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeBlur, Color: red}, [2]uint8{190, 220}, [2]uint8{0, 16}},
		// The background at 93% over the blur and 85% over the glass, and
		// nothing over bare glass or a transparent bar.
		{ScrollEdgeBarOptions{Style: ScrollEdgeBlur}, [2]uint8{180, 205}, [2]uint8{180, 205}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeGlass}, [2]uint8{220, 250}, [2]uint8{220, 250}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeGlass, Background: red}, [2]uint8{220, 255}, [2]uint8{0, 45}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeGlass, Color: red, Uncolored: true}, [2]uint8{0, 150}, [2]uint8{0, 150}},
		{ScrollEdgeBarOptions{Style: ScrollEdgeBlur, Uncolored: true}, [2]uint8{0, 16}, [2]uint8{0, 16}},
	} {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(false))
			ui.Box(c).Fill().Clip().Children(func() {
				ui.Box(c).Fill().Background(ui.RGB(0, 0, 0))
				ScrollEdgeBar(c, tc.options, func() {
					Button(c, "Action", ButtonSecondary)
				})
			})
		}, 300, 300)
		img := tt.Image()
		s := float32(img.Bounds().Dx()) / 300
		px := img.RGBAAt(int(240*s), int(38*s))
		if px.R < tc.r[0] || px.R > tc.r[1] || px.G < tc.g[0] || px.G > tc.g[1] {
			t.Fatalf("%+v: bar %v", tc.options, px)
		}
	}
}
