package phone

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// ToastOptions returns o with the window's covered edge added to its Inset,
// so the toast stack clears the system bar its placement meets. The shells
// alone apply the safe area; components never handle insets.
func ToastOptions(c *ui.Context, o components.ToastOptions) components.ToastOptions {
	top, _, bottom, _ := c.SafeArea()
	if o.Placement.Top() {
		o.Inset += top
	} else {
		o.Inset += bottom
	}
	return o
}
