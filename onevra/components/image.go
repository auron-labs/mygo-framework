package components

import "github.com/egoist/mygo/ui"

// Image displays a decoded *ui.Bitmap or *ui.SVG in its own colors.
// Applications own loading, decoding, caching and errors; reuse decoded assets
// rather than performing file or network I/O in a content builder.
//
// Images default to their intrinsic size in DIPs and contain-fit when resized.
// Use Size, Fit, Radius and Grayscale on the returned element to customize its
// presentation, and Label for application-owned accessible text. A nil source
// produces an empty element. MyGo owns rendering, layout and image semantics.
func Image(c *ui.Context, source ui.ImageSource) ui.Element {
	return ui.Image(c, source)
}
