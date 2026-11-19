// Package lucide provides Lucide icons for MyGo without a separate widget layer.
//
// Use components.Icon(c, lucide.Search) or pass an icon to ui.SidebarItem. MyGo owns
// sizing, text-color inheritance, rendering and accessibility. Icons retain
// Lucide's original strokes and scale with the element; variable stroke width
// and absolute stroke width are not provided.
//
// Each accessor lazily parses its SVG and returns the same shared pointer.
// There is no runtime registry: only tests reference the entire collection.
// Names follow upstream kebab-case converted to PascalCase (for example,
// arrow-up-right becomes ArrowUpRight); names starting with a digit use Icon
// as a prefix. Upstream aliases and lab icons are not included.
//
// Generated assets are pinned in upstream.json and covered by the adjacent
// Lucide/Feather LICENSE, rather than the repository's own MIT license.
package lucide

import (
	"sync"

	"github.com/egoist/mygo/ui"
)

// Zero-value caches avoid package initializers rooting unused SVG resources.
type lazyIcon struct {
	once sync.Once
	svg  *ui.SVG
}

func (i *lazyIcon) get(data string) *ui.SVG {
	i.once.Do(func() { i.svg = ui.MustParseSVG([]byte(data)) })
	return i.svg
}
