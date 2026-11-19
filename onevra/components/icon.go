package components

import "github.com/egoist/mygo/ui"

// Icon creates an SVG icon from a parsed asset or a named accessor such as
// lucide.Search (without calling it). Accessors run synchronously when the
// element is built; the asset package owns parsing and caching. A typed nil
// SVG or accessor produces an empty icon.
//
// Icons inherit text color and font size from their parent. Use Size to set
// explicit dimensions and Label to supply application-owned accessible text.
// MyGo owns rendering, aspect ratio and element semantics. No icon registry or
// dependency on a particular icon pack is introduced.
func Icon[T *ui.SVG | func() *ui.SVG](c *ui.Context, source T) ui.Element {
	var svg *ui.SVG
	switch value := any(source).(type) {
	case *ui.SVG:
		svg = value
	case func() *ui.SVG:
		if value != nil {
			svg = value()
		}
	}
	return ui.Icon(c, svg)
}
