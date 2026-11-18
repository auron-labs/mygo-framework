package components

import (
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// Toast types for ui.Toast.Type select a colored leading status icon. Other
// types, including the empty one, show no icon.
const (
	ToastInfo    = "info"
	ToastSuccess = "success"
	ToastWarning = "warning"
	ToastError   = "error"
)

// ToastContent is optional ui.Toast.Data for content slots. Leading replaces
// the type's status icon, for example with an image or a progress indicator; it
// runs synchronously while Toasts builds the toast.
type ToastContent struct {
	Leading func()
}

// ToastPlacement selects the window edge and side where toasts stack. The zero
// value stacks at the top center. Top placements put the newest toast first,
// nearest the top edge, and stack older ones downward; bottom placements put it
// last, nearest the bottom edge.
type ToastPlacement int

const (
	ToastTop ToastPlacement = iota
	ToastTopStart
	ToastTopEnd
	ToastBottom
	ToastBottomStart
	ToastBottomEnd
)

// Top reports whether p places toasts at the top edge.
func (p ToastPlacement) Top() bool { return p >= ToastTop && p <= ToastTopEnd }

func (p ToastPlacement) align() ui.Align {
	switch p {
	case ToastBottomStart, ToastTopStart:
		return ui.Start
	case ToastBottomEnd, ToastTopEnd:
		return ui.End
	}
	return ui.Center
}

// ToastOptions customizes where Toasts places the window's toasts. The zero
// value stacks at the top center. Inset adds DIPs at the placement's edge beyond
// the standard margin, for example to clear a title bar; Shell adds its header
// height to top placements itself.
type ToastOptions struct {
	Placement ToastPlacement
	Inset     float32
}

// Toasts builds the window's MyGo toasts in the Onevra style, replacing MyGo's
// default look, and returns the viewport. Applications show and close toasts
// with MyGo's Context.Toast, ToastAction, AddToast and CloseToast; MyGo owns the
// queue, timing, pausing, Escape dismissal and announcements.
//
// Shell calls Toasts after its content with ShellOptions.Toasts, so
// applications call it only in standalone hosts; the first options value
// applies. A view that builds its own ToastViewportBase earlier in the frame
// keeps its look. Toasts stack at the top center by default, each with an
// optional action and a close button named by the "onevra.closeToast" message.
// Older toasts tuck behind the newest one, each peeking out a little further
// and drawn a little smaller, and fan out while the pointer rests on the stack
// or the keyboard focus is in it.
func Toasts(c *ui.Context, options ...ToastOptions) ui.Element {
	var o ToastOptions
	if len(options) > 0 {
		o = options[0]
	}
	t := c.Theme()
	closeLabel := localized(c).CloseToast
	width, _ := c.Size()
	margin, inset := t.Space(4), max(0, o.Inset)
	top := o.Placement.Top()
	// dir points from the placement's edge into the window.
	dir, origin := float32(-1), float32(0)
	if top {
		dir, origin = 1, 1
	}
	gap, peek := t.Space(2), t.Space(3.5)
	padY, border := t.Space(3), float32(1)
	spring := ui.Snappy
	motion := ui.ElementTransition{
		Animation: &spring, Size: true,
		Enter: &ui.Motion{Y: -dir * t.Space(16)}, Exit: &ui.Motion{Scale: .96},
	}
	return ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.AlignItems(o.Placement.align())
		if top {
			viewport.Justify(ui.Start).Padding(margin+inset, margin, margin, margin)
		} else {
			viewport.Padding(margin, margin, margin+inset, margin)
		}
		// The stack takes the pointer across the gaps between fanned-out
		// toasts, so that crossing them keeps the stack open.
		stack := ui.Box(c).Key("onevra.toasts").Width(min(max(0, width-2*margin), t.Space(90)))
		expanded := stack.Hovered() || stack.FocusWithin()
		n := len(toasts)
		roots := make([]ui.Element, n)
		// Each toast's own height comes from the previous frame's layout of
		// its content, which a tucked toast's fixed height does not change.
		heights := make([]float32, n)
		stack.Children(func() {
			// Older toasts come first, under the newer ones.
			for i, toast := range toasts {
				parts := ui.ToastBase(c, toast)
				roots[i] = parts.Root.Absolute().Left(0).Right(0).Clip().
					Padding(padY, t.Space(2), padY, t.Space(4)).Radius(t.Radius*1.5).
					Material(glass.Glass{Tint: t.Background.Alpha(.92)}).TextColor(t.Text).Border(border, t.Border).
					Shadow(0, t.Space(1), t.Space(4), 0, ui.RGBA(0, 0, 0, .14)).
					Transition(motion).TransformOrigin(.5, origin)
				if top {
					roots[i].Top(0)
				} else {
					roots[i].Bottom(0)
				}
				fade := float32(0)
				if i < n-1 && !expanded {
					fade = 1
				}
				roots[i].Children(func() {
					content := toastContent(c, parts, toast, closeLabel, fade, spring)
					if h := content.LayoutBounds().H; h > 0 {
						heights[i] = h + 2*padY + 2*border
					}
				})
			}
		})
		// Tucked toasts take the newest one's height, or until a toast that
		// just arrived has been laid out, the next newest's.
		var front float32
		for i := n - 1; i >= 0 && front == 0; i-- {
			front = heights[i]
		}
		stackHeight := front + peek*float32(max(0, n-1))
		var offset float32
		for i := n - 1; i >= 0; i-- {
			depth := float32(n - 1 - i)
			pose := ui.Pose{Y: dir * offset}
			offset += heights[i] + gap
			if depth > 0 && !expanded {
				pose = ui.Pose{Y: dir * peek * depth, Scale: 1 - .05*depth}
				if front > 0 {
					roots[i].Height(front)
				}
			}
			roots[i].AnimatePose(pose, spring)
		}
		if expanded {
			stackHeight = max(0, offset-gap)
		}
		stack.Height(stackHeight)
	})
}

// toastContent builds the toast's content in a row, which fades out while the
// toast is tucked behind another, and returns the row.
func toastContent(c *ui.Context, parts ui.ToastParts, toast ui.Toast, closeLabel string, fade float32, a ui.Animation) ui.Element {
	t := c.Theme()
	align := ui.Center
	if toast.Description != "" {
		align = ui.Start
	}
	return ui.Row(c).Shrink(0).AlignItems(align).Gap(t.Space(3)).AnimatePose(ui.Pose{Fade: fade}, a).Children(func() {
		toastLeading(c, toast)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(t.Space(.5)).Children(func() {
			Text(c, toast.Title).FontWeight(500)
			if toast.Description != "" {
				Text(c, toast.Description).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
			}
		})
		if toast.Action != "" {
			styleButton(c, parts.ActionButton(), ButtonSecondary).Shrink(0).
				Label(toast.Action).Children(func() { Text(c, toast.Action).SingleLine() })
		}
		size := t.Space(6)
		styleButton(c, parts.CloseButton(), ButtonGhost).Label(closeLabel).
			Size(size, size).Padding(0).Shrink(0).TextColor(t.TextMuted).Children(func() {
			Icon(c, lucide.X).Size(t.Rem(.875), t.Rem(.875))
		})
	})
}

func toastLeading(c *ui.Context, toast ui.Toast) {
	if content, ok := toast.Data.(ToastContent); ok && content.Leading != nil {
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(content.Leading)
		return
	}
	t := c.Theme()
	var icon func() *ui.SVG
	var color ui.Color
	switch toast.Type {
	case ToastInfo:
		icon, color = lucide.Info, t.Accent
	case ToastSuccess:
		icon, color = lucide.CircleCheck, t.Success
	case ToastWarning:
		icon, color = lucide.TriangleAlert, t.Warning
	case ToastError:
		icon, color = lucide.CircleX, t.Danger
	default:
		return
	}
	size := t.Rem(1.125)
	Icon(c, icon).Size(size, size).Shrink(0).TextColor(color).Role(ui.RoleNone)
}
