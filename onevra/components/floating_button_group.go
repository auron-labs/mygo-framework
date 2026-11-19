package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// FloatingButtonGroupVariant selects the tray of a FloatingButtonGroup.
type FloatingButtonGroupVariant int

const (
	// FloatingButtonGroupMuted sits quietly on a page or panel, on a
	// translucent tint of the text color. This is the default.
	FloatingButtonGroupMuted FloatingButtonGroupVariant = iota
	// FloatingButtonGroupRaised lifts the tray on the raised surface with a
	// hairline edge and a floating shadow, for toolbars over content such as
	// a canvas or a text selection.
	FloatingButtonGroupRaised
)

// FloatingButtonGroupItem is a button, or a separator, of a
// FloatingButtonGroup.
type FloatingButtonGroupItem struct {
	// ID identifies the button across frames, so its label can change, and
	// is what Clicked reports.
	ID string
	// Label is the visible text and the accessible name; IconOnly moves it
	// into a tooltip.
	Label string
	Icon  func() *ui.SVG
	// IconOnly shows only Icon, when there is one.
	IconOnly bool
	Disabled bool
	// Toggle marks an action that switches a mode, such as Present, whose
	// state On holds: assistive technology sees a toggle button, and a
	// quiet tint marks it while on.
	Toggle, On bool
	// Shortcut is a key hint, such as "⌘Z", shown in the tooltip after
	// Label. The group does not bind the key.
	Shortcut string
	// ReserveLabels lists the other labels the button can switch to, such
	// as "Copied" for "Share". The button keeps the widest one's width, so
	// a label change never moves anything.
	ReserveLabels []string
	// Separator makes the item a short rule between clusters of buttons;
	// its other fields are ignored.
	Separator bool
}

// FloatingButtonGroupControl is the tray of a FloatingButtonGroup.
type FloatingButtonGroupControl struct {
	ui.Element
	clicked string
}

// Clicked returns the ID of the button activated in this frame, or "".
func (g *FloatingButtonGroupControl) Clicked() string { return g.clicked }

// floatingGroupState is what a FloatingButtonGroup keeps between frames.
type floatingGroupState struct {
	// hover is the button the pointer was last over, kept while it crosses
	// gaps and separators, until it leaves the tray.
	hover string
	// focus is the button with keyboard focus; keyboard tells that it took
	// the highlight from the pointer, until the pointer moves again.
	focus    string
	keyboard bool
	px, py   float32
	// shown tells that the highlight is on a button, so moving travels
	// rather than appearing in place.
	shown bool
	// What the last build pass painted, for paint-only passes.
	highlight ui.Rect
	scale     float32
	color     ui.Color
	tints     []ui.Rect
}

var (
	floatingGroupMorph  = ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond, Bounce: 0.12}}
	floatingGroupSnappy = ui.Animation{Spring: ui.SpringOptions{Duration: 250 * time.Millisecond, Bounce: 0.15}}
	floatingGroupFade   = ui.Animation{Duration: 140 * time.Millisecond}
)

// FloatingButtonGroup creates a toolbar of separate soft buttons in a quiet
// tray, laid out along orientation, ui.Horizontal or ui.Vertical; label
// names it for assistive technology. Use it for actions; for a choice that
// stays selected use Tabs or a toggle.
//
// One shared highlight marks the button under the pointer, or with keyboard
// focus, and springs between buttons in position and size. It stays put as
// the pointer crosses a gap or separator and fades as it leaves the tray.
// Appearing, it grows in place; a press darkens it and settles it slightly
// smaller, while the buttons themselves never move. A changed label or icon
// crossfades in place. Reduced Motion moves the highlight at once.
//
// The tray is one Tab stop; the arrows of its orientation move among the
// enabled buttons, wrapping at the ends, Home and End go to the ends, and
// Enter or Space activates. Icon-only buttons and buttons with a Shortcut
// show a tooltip, above a row and beside a column. A column's buttons share
// the width of its widest one. Applications own the
// labels, the state of toggles and the actions; ask Clicked for the
// activated button.
func FloatingButtonGroup(c *ui.Context, label string, variant FloatingButtonGroupVariant, orientation ui.Orientation, items []FloatingButtonGroupItem) *FloatingButtonGroupControl {
	t := c.Theme()
	vertical := orientation == ui.Vertical
	size, pad := max(32, t.Rem(2.25)), t.Space(1)
	radius := max(0, t.Radius-2)
	var tray ui.Element
	if vertical {
		tray = ui.Column(c).AlignItems(ui.Stretch)
	} else {
		tray = ui.Row(c).AlignItems(ui.Center)
	}
	tray.Shrink(0).AlignSelf(ui.Start).Padding(pad).Gap(t.Space(1)).Radius(radius + pad + 1).
		Role(ui.RoleToolbar).Label(label).FocusGroup(orientation)
	tint, edge := t.Text.Alpha(.045), t.Text.Alpha(.03)
	if t.Dark {
		tint, edge = t.Text.Alpha(.06), t.Text.Alpha(.05)
	}
	if variant == FloatingButtonGroupRaised {
		face, _, _, shadow := toolbarFace(t)
		tint, edge = face, t.Border
		tray.Shadow(0, t.Space(2), t.Space(6), 0, shadow.Alpha(.85))
	}
	tray.Background(tint).Border(1, edge)

	g := &FloatingButtonGroupControl{Element: tray}
	st := ui.Local(tray, "floating-button-group", func() floatingGroupState { return floatingGroupState{scale: 1} })
	px, py, in := tray.PointerPosition()
	if in && (px != st.px || py != st.py) {
		st.keyboard = false
	}
	st.px, st.py = px, py

	type built struct {
		item                      FloatingButtonGroupItem
		button                    ui.Element
		iconOnly, focus, disabled bool
	}
	var buttons []built
	over, focus, pressed := "", "", ""
	tray.Children(func() {
		// Buttons come first, without content, so the highlight's button is
		// known before any of them takes its colors.
		for _, item := range items {
			if item.Separator {
				rule := ui.Box(c).Shrink(0).AlignSelf(ui.Center).Background(t.Text.Alpha(.12)).Radius(.5)
				if vertical {
					rule.Size(size*.5, 1).MarginY(t.Space(.5))
				} else {
					rule.Size(1, size*.5).MarginX(t.Space(.5))
				}
				continue
			}
			b := ui.ButtonBase(c).Key(item.ID).Label(item.Label).Disabled(item.Disabled).
				Height(size).MinWidth(size).Shrink(0).Radius(radius).FocusRing(false).Gap(t.Space(1.5))
			if item.Toggle {
				b.Role(ui.RoleToggleButton).Checked(item.On)
			}
			if b.Clicked() {
				g.clicked = item.ID
			}
			if _, _, ok := b.PointerPosition(); ok {
				over = item.ID
			}
			it := built{item: item, button: b, iconOnly: item.IconOnly && item.Icon != nil, focus: b.FocusVisible(), disabled: b.IsDisabled()}
			if it.focus {
				focus = item.ID
			}
			if b.Pressed() {
				pressed = item.ID
			}
			buttons = append(buttons, it)
		}
	})

	switch {
	case !in:
		st.hover = ""
	case over != "":
		st.hover = over
	}
	if focus != "" && focus != st.focus {
		st.keyboard = true
	}
	st.focus = focus
	active := ""
	for _, it := range buttons {
		if it.item.ID == st.hover && !it.disabled && !st.keyboard {
			active = it.item.ID
		}
	}
	if active == "" {
		active = focus
	}

	var target ui.Element
	var on []ui.Element
	for _, it := range buttons {
		item, b := it.item, it.button
		if item.ID == active {
			target = b
		}
		if item.Toggle && item.On {
			on = append(on, b)
		}
		fg := t.TextMuted
		if item.ID == active || item.Toggle && item.On {
			fg = t.Text
		}
		b.TextColor(fg).FontWeight(500)
		switch {
		case it.iconOnly:
			// A column's buttons share its width, so the highlight keeps one
			// width down the rail.
			b.Padding(0)
			if !vertical {
				b.Width(size)
			}
		case item.Icon != nil:
			b.Padding(0, t.Space(3), 0, t.Space(2.5))
		default:
			b.Padding(0, t.Space(3))
		}
		if vertical && !it.iconOnly {
			b.Justify(ui.Start)
		}
		if it.focus {
			b.DrawOver(func(p *ui.Painter, r ui.Rect) {
				p.FocusRing(r, [4]float32{radius, radius, radius, radius})
			})
		}
		b.Children(func() { floatingButtonBody(c, item, it.iconOnly, vertical) })
		dimmed(b)
		if it.iconOnly || item.Shortcut != "" {
			floatingButtonTip(c, b, item, vertical)
		}
	}

	press := active != "" && active == pressed
	tray.Draw(func(p *ui.Painter, r ui.Rect) {
		if tray.Valid() {
			placeHighlight(c, tray, st, r, target, on, press)
		}
		for _, tint := range st.tints {
			p.Fill(ui.Rect{X: r.X + tint.X, Y: r.Y + tint.Y, W: tint.W, H: tint.H}, t.Accent.Alpha(.14), radius)
		}
		if h := st.highlight; st.color.A > 0 && h.W > 0 {
			w, hh := h.W*st.scale, h.H*st.scale
			rect := ui.Rect{X: r.X + h.X + (h.W-w)/2, Y: r.Y + h.Y + (h.H-hh)/2, W: w, H: hh}
			p.Fill(rect, st.color, radius*st.scale)
		}
	})
	return g
}

// placeHighlight moves the shared highlight of tray, whose rectangle is r,
// to target, or fades it without one, and notes the toggled buttons' tints.
// It runs while painting the build pass, once layout is known.
func placeHighlight(c *ui.Context, tray ui.Element, st *floatingGroupState, r ui.Rect, target ui.Element, on []ui.Element, press bool) {
	t := c.Theme()
	st.tints = st.tints[:0]
	for _, b := range on {
		bounds := b.LayoutBounds()
		st.tints = append(st.tints, ui.Rect{X: bounds.X - r.X, Y: bounds.Y - r.Y, W: bounds.W, H: bounds.H})
	}
	show := float32(0)
	if target.Valid() {
		show = 1
	}
	opacity := tray.AnimateTo("highlight-opacity", show, floatingGroupFade)
	if target.Valid() {
		bounds := target.LayoutBounds()
		to := ui.Rect{X: bounds.X - r.X, Y: bounds.Y - r.Y, W: bounds.W, H: bounds.H}
		if !st.shown && opacity < .15 {
			// Appearing from nothing lands in place and grows in, rather
			// than sliding from wherever the highlight last was.
			tray.Jump("highlight-x", to.X)
			tray.Jump("highlight-y", to.Y)
			tray.Jump("highlight-w", to.W)
			tray.Jump("highlight-h", to.H)
			if !c.Preferences().ReduceMotion {
				tray.Jump("highlight-scale", .94)
			}
		}
		st.shown = true
		st.highlight = ui.Rect{
			X: tray.AnimateTo("highlight-x", to.X, floatingGroupMorph),
			Y: tray.AnimateTo("highlight-y", to.Y, floatingGroupMorph),
			W: tray.AnimateTo("highlight-w", to.W, floatingGroupMorph),
			H: tray.AnimateTo("highlight-h", to.H, floatingGroupMorph),
		}
	} else {
		st.shown = false
	}
	scale := float32(1)
	face := t.Text.Alpha(.07)
	if t.Dark {
		face = t.Text.Alpha(.09)
	}
	if press && target.Valid() {
		scale = .965
		face = t.Text.Alpha(.12)
		if t.Dark {
			face = t.Text.Alpha(.15)
		}
	}
	st.scale = tray.AnimateTo("highlight-scale", scale, floatingGroupSnappy)
	st.color = tray.AnimateColor("highlight-color", face, ui.Animation{Duration: 100 * time.Millisecond}).Alpha(opacity)
}

// floatingButtonBody builds a button's icon and label. Every label the
// button can show is laid out invisibly in the label's place, so it keeps
// the widest one's width.
func floatingButtonBody(c *ui.Context, item FloatingButtonGroupItem, iconOnly, vertical bool) {
	t := c.Theme()
	if item.Icon != nil {
		Icon(c, item.Icon).Size(t.Rem(1), t.Rem(1)).Shrink(0).ContentTransition(ui.ContentTransition{})
	}
	if iconOnly {
		return
	}
	slot := ui.Column(c).AlignItems(ui.Center)
	if vertical {
		slot.AlignItems(ui.Start)
	}
	slot.Children(func() {
		Text(c, item.Label).SingleLine().ContentTransition(ui.ContentTransition{})
		for _, reserve := range item.ReserveLabels {
			if reserve != item.Label {
				Text(c, reserve).SingleLine().Height(0).Invisible()
			}
		}
	})
}

// floatingButtonTip attaches a button's tooltip: its label, when only its
// icon shows, and its shortcut. A column's tips open beside it, so they do
// not cover the neighbouring buttons.
func floatingButtonTip(c *ui.Context, b ui.Element, item FloatingButtonGroupItem, vertical bool) {
	t := c.Theme()
	if item.Shortcut != "" {
		b.Description(item.Shortcut)
	}
	ui.TooltipBase(c, b, func(tip ui.Element) {
		if vertical {
			tip.AttachTo(b, ui.AnchorEnd, ui.AnchorStart).MarginStart(t.Space(2.5))
		}
		styleTip(c, tip).Row().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			Text(c, item.Label).SingleLine()
			if item.Shortcut != "" {
				Text(c, item.Shortcut).SingleLine().TextColor(t.Background.Mix(t.Text, .42))
			}
		})
	})
}
