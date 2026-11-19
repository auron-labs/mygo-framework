package components

import "github.com/egoist/mygo/ui"

type dialogWindowDragKey struct{}

func dialogWindowDrag(c *ui.Context) *float32 {
	return ui.Local(c.Root(), dialogWindowDragKey{}, func() float32 { return 0 })
}

type dialogDepthKey struct{}

// dialogDepth counts the Dialog builders running, so nested layers know
// they are nested.
func dialogDepth(c *ui.Context) *int {
	return ui.Local(c.Root(), dialogDepthKey{}, func() int { return 0 })
}

// WithDialogWindowDrag supplies the default native window drag strip height
// in DIPs for Dialog and AlertDialog calls made synchronously by build.
// Shell installs this scope automatically; standalone hosts may use it too.
// Non-positive heights disable the default. Nested scopes replace the height
// and restore it on return. This does not add layout or forward pointer events.
// Call during the UI build; do not retain c or launch asynchronous builders.
func WithDialogWindowDrag(c *ui.Context, height float32, build func()) {
	current := dialogWindowDrag(c)
	previous := *current
	*current = max(0, height)
	defer func() { *current = previous }()
	if build != nil {
		build()
	}
}
