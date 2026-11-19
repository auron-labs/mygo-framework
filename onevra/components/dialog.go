package components

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// DialogOptions controls dismissal independently for each layer.
// The zero value allows backdrop clicks and Escape to dismiss the dialog.
type DialogOptions struct {
	DisableBackdropClose bool
	DisableEscapeClose   bool
	// Label is the accessible name. Applications supply localized text.
	Label string
	// Inset is the minimum viewport clearance in DIPs on every edge.
	// Non-positive values use 20. WindowDragHeight can increase this clearance.
	Inset float32
	// WindowDragHeight overrides the host's native window drag strip height
	// in DIPs. Non-positive values inherit WithDialogWindowDrag (zero without
	// a host). MyGo owns native drag and focus behavior.
	WindowDragHeight float32
	// DisableWindowDrag disables the strip, including an explicit height.
	DisableWindowDrag bool
}

// Dialog builds arbitrary content in a modal panel and returns its frame-local
// handle (absent when closed). Applications own open and call Dialog every frame.
// Build layers in back-to-front order, with stable keyed parents for independent
// dialogs. Nested calls are supported. MyGo owns focus trapping and restoration.
// The panel is horizontally centered with spare vertical space biased below it.
// Its default maximum size leaves 20 logical pixels clear on every viewport edge;
// Inset overrides the clearance; WindowDragHeight can increase it.
// Content may customize the panel and should provide scrolling for long pages.
// The returned panel does not report Dismissed; use DisableBackdropClose and
// DisableEscapeClose to keep a layer open.
func Dialog(c *ui.Context, open *bool, options DialogOptions, build func(panel ui.Element)) ui.Element {
	var panel ui.Element
	if !*open {
		// Keep the overlay root alive so the next opening animates in and a
		// layer that just closed has a parent for its exit.
		ui.Overlay(c, func() {})
		return panel
	}
	t := c.Theme()
	w, h := c.Size()
	// Distribute spare vertical space 1:2 above/below the panel.
	inset := options.Inset
	if inset <= 0 {
		inset = 20
	}
	dragHeight := options.WindowDragHeight
	if dragHeight <= 0 {
		dragHeight = *dialogWindowDrag(c)
	}
	if options.DisableWindowDrag {
		dragHeight = 0
	}
	dragHeight = min(max(0, dragHeight), max(0, h))
	inset = max(inset, dragHeight)
	// MyGo paints an exiting copy under its overlay siblings, so a nested
	// layer closing above its parent would drop behind it: only outermost
	// layers animate out.
	depth := dialogDepth(c)
	fade, pop := dialogFade, dialogPop
	if *depth > 0 {
		fade.Exit, pop.Exit = nil, nil
	}
	ui.Overlay(c, func() {
		// The dim and the panel layer are separate so the panel can scale
		// without revealing the dim's edges.
		ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Background(ui.RGBA(0, 0, 0, .18)).Transition(fade)
		back := ui.Column(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			AlignItems(ui.Center).Padding(inset).Modal().Transition(pop)
		clicked := back.Clicked()
		escape := back.OverlayShortcut(0, ui.KeyEscape)
		if clicked && !options.DisableBackdropClose || escape && !options.DisableEscapeClose {
			*open = false
		}
		back.Children(func() {
			ui.Box(c).Grow(1).MinHeight(0)
			panel = ui.Column(c).Role(ui.RoleDialog).Label(options.Label).
				Width(t.Space(110)).MaxWidth(max(0, w-2*inset)).MaxHeight(max(0, h-2*inset)).Shrink(0).
				Padding(t.Space(5)).Gap(t.Space(3)).Radius(t.Radius*2).
				Material(glass.Glass{Tint: t.Background.Alpha(.96)}).TextColor(t.Text).
				Border(1, t.Background.Mix(t.Text, .25)).
				Shadow(0, t.Space(2), t.Space(6), 0, ui.RGBA(0, 0, 0, .12))
			// Register a hit target so blank panel space never dismisses the backdrop.
			panel.Clicked()
			panel.Children(func() {
				if build != nil {
					*depth++
					defer func() { *depth-- }()
					build(panel)
				}
			})
			ui.Box(c).Grow(2).MinHeight(0)
			if dragHeight > 0 {
				// A dedicated hit target, not a clickable backdrop or a forwarded
				// event: MyGo starts the native drag without dismissing this layer.
				ui.Box(c).Absolute().Left(0).Top(0).Right(0).Height(dragHeight).DragWindow()
			}
		})
	})
	return panel
}

// Dialog layers enter and exit through MyGo transitions, which keep only
// the fades under Reduced Motion.
var (
	dialogFade = ui.ElementTransition{
		Duration: 180 * time.Millisecond, Colors: true,
		Enter: &ui.Motion{Opacity: 0}, Exit: &ui.Motion{Opacity: 0},
	}
	dialogPop = ui.ElementTransition{
		Duration: 180 * time.Millisecond, Colors: true,
		Enter: &ui.Motion{Opacity: 0, Scale: .96, Y: 8}, Exit: &ui.Motion{Opacity: 0, Scale: .98},
	}
)

// DialogResult is emitted only in the frame of a user action. Store it in
// application state if needed later; this UI-thread API never blocks.
type DialogResult uint8

const (
	DialogNone DialogResult = iota
	DialogConfirmed
	DialogCancelled
)

// AlertDialogOptions supplies localized text and optional cancellation.
// ConfirmLabel must be supplied. HideCancel produces a one-button notice;
// backdrop and Escape dismissal still return DialogCancelled unless disabled.
type AlertDialogOptions struct {
	DialogOptions
	Title, Message            string
	ConfirmLabel, CancelLabel string
	HideCancel                bool
}

// AlertDialog is a preset Dialog returning confirmation or cancellation.
// Programmatically closing open does not emit a result.
func AlertDialog(c *ui.Context, open *bool, options AlertDialogOptions) DialogResult {
	result := DialogNone
	wasOpen := *open
	opts := options.DialogOptions
	opts.Label = options.Title
	Dialog(c, open, opts, func(panel ui.Element) {
		panel.Role(ui.RoleAlertDialog)
		if options.Title != "" {
			Text(c, options.Title).Bold()
		}
		if options.Message != "" {
			Text(c, options.Message).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(c.Theme().Space(2)).Children(func() {
			if !options.HideCancel && Button(c, options.CancelLabel, ButtonSecondary).Clicked() {
				result = DialogCancelled
				*open = false
			}
			if Button(c, options.ConfirmLabel, ButtonPrimary).Clicked() {
				result = DialogConfirmed
				*open = false
			}
		})
	})
	if wasOpen && !*open && result == DialogNone {
		result = DialogCancelled
	}
	return result
}
