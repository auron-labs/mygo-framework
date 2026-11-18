package components

import (
	"math"
	"strconv"
	"strings"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// NumberInputControl exposes layout on Element and editing on the text field.
// Keep this handle to query Changed after applying Element layout methods.
type NumberInputControl struct {
	ui.Element
	input, increase, decrease, grip ui.Element
	changed                         bool
	words                           localization
}

// Changed reports an input-driven value change, not initial clamping.
func (n *NumberInputControl) Changed() bool { return n.changed }

// Label names the text field and any optional drag handle and step buttons.
func (n *NumberInputControl) Label(label string) *NumberInputControl {
	n.input.Label(label)
	n.grip.Label(n.words.numberAction("adjust", label))
	n.increase.Label(n.words.numberAction("increase", label))
	n.decrease.Label(n.words.numberAction("decrease", label))
	return n
}

// AutoFocus focuses the editable text field.
func (n *NumberInputControl) AutoFocus() *NumberInputControl {
	n.input.AutoFocus()
	return n
}

type numberInputState struct {
	text  string
	value float64
	held  bool
	scrub float64
}

// NumberInputOptions configures optional controls around the editor.
type NumberInputOptions struct {
	// LeadingText and LeadingIcon provide a horizontal drag handle inside the field.
	// Dragging changes the value by step per theme-scaled DIP. Arrow keys change
	// it by step; Home/End select the bounds. Omit both for an ordinary text field.
	LeadingText string
	LeadingIcon *ui.SVG
	// HideStepper hides the right-hand up/down buttons, which are shown by default.
	HideStepper bool
}

// NumberInput creates a numeric text field with optional prefix scrubbing and
// an optional vertical stepper. Bounds and value must be finite, hi >= lo,
// and step finite and positive.
// Valid in-range text updates value immediately without snapping to step. Partial
// or invalid text is retained while editing and restored on Enter or focus loss.
// Up/Down and the buttons add/subtract step and clamp to the bounds. Application
// changes replace even a focused draft. Disable a parent to disable the control.
// Key a parent for stable identity; construction resolves interaction state.
func NumberInput(c *ui.Context, value *float64, lo, hi, step float64, options ...NumberInputOptions) *NumberInputControl {
	if len(options) > 1 {
		panic("components.NumberInput: at most one options value required")
	}
	var opts NumberInputOptions
	if len(options) == 1 {
		opts = options[0]
	}
	return numberInput(c, value, lo, hi, step, opts, nil, step, step)
}

// The custom editor preserves color-specific draft validation and percent
// notation while sharing layout, accessibility, stepping and pointer scrubbing.
func numberInput(c *ui.Context, value *float64, lo, hi, step float64, opts NumberInputOptions, editor func() ui.Element, dragScale, dragStep float64) *NumberInputControl {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if value == nil || !finite(*value) || !finite(lo) || !finite(hi) || hi < lo || !finite(step) || step <= 0 {
		panic("components.NumberInput: finite value and ordered bounds, and positive finite step required")
	}
	*value = max(lo, min(hi, *value))
	t := c.Theme()
	n := &NumberInputControl{words: localized(c), Element: ui.Row(c).AlignItems(ui.Stretch).Width(t.Space(32)).Role(ui.RoleGroup)}
	format := func() string { return strconv.FormatFloat(*value, 'f', -1, 64) }
	state := ui.Local(n.Element, "number-input", func() numberInputState {
		return numberInputState{text: format(), value: *value}
	})
	if state.value != *value {
		state.text = format()
		state.held = false
	}
	set := func(v float64) {
		v = max(lo, min(hi, v))
		if *value != v {
			*value = v
			n.changed = true
		}
	}
	advance := func(delta float64) {
		// Decimal arithmetic through shortest decimal representations avoids
		// accumulating binary tails for ordinary fractional steps.
		v := *value + delta
		precision := func(x float64) int {
			s := strconv.FormatFloat(x, 'f', -1, 64)
			if i := strings.IndexByte(s, '.'); i >= 0 {
				return len(s) - i - 1
			}
			return 0
		}
		p := math.Pow10(max(precision(*value), precision(step)))
		if finite(v*p) && finite(p) {
			v = math.Round(v*p) / p
		}
		set(v)
		state.text = format()
	}
	n.Element.Children(func() {
		ui.Box(c).FillWidth().MinWidth(0).Children(func() {
			if editor != nil {
				n.input = editor()
			} else {
				n.input = TextBox(c, &state.text, TextBoxOptions{LeadingText: opts.LeadingText, LeadingIcon: opts.LeadingIcon}).FillWidth().MinWidth(0)
				if n.input.Changed() {
					if v, err := strconv.ParseFloat(strings.TrimSpace(state.text), 64); err == nil && finite(v) && v >= lo && v <= hi {
						set(v)
					}
				}
				if n.input.Shortcut(0, ui.KeyUp) {
					advance(step)
				}
				if n.input.Shortcut(0, ui.KeyDown) {
					advance(-step)
				}
				if !n.input.Focused() || n.input.Submitted() {
					state.text = format()
				}
			}
			width, _ := c.MeasureText(0, ui.Span{Text: opts.LeadingText})
			width += t.Space(2.5)
			if opts.LeadingText != "" {
				width += t.Space(2)
			}
			if opts.LeadingIcon != nil {
				width += t.Space(5.5)
			}
			if !opts.HideStepper {
				n.input.Padding(t.Space(1.5), t.Space(6)+1, t.Space(1.5), width)
			}
			if opts.LeadingText != "" || opts.LeadingIcon != nil {
				n.grip = ui.Box(c).Absolute().Left(1).Top(1).Bottom(1).Width(width).
					Cursor(ui.CursorResizeEW).Focusable().Role(ui.RoleSlider)
				dx, _, held := n.grip.Dragged()
				if !n.grip.IsDisabled() {
					if held {
						n.grip.Focus()
						if !state.held {
							state.scrub = *value
						}
						state.scrub = max(lo, min(hi, state.scrub+float64(dx/t.Space(.25))*dragScale))
						if dx != 0 {
							v := state.scrub
							// Dividing by the reciprocal avoids binary tails for
							// ordinary decimal steps. Extreme finite steps can
							// overflow the quotient; retain the bounded value.
							p := 1 / dragStep
							if finite(p) && p > 0 && finite(v*p) {
								v = math.Round(v*p) / p
							}
							set(v)
							state.text = format()
						}
					}
					switch {
					case n.grip.Shortcut(0, ui.KeyRight), n.grip.Shortcut(0, ui.KeyUp):
						advance(step)
					case n.grip.Shortcut(0, ui.KeyLeft), n.grip.Shortcut(0, ui.KeyDown):
						advance(-step)
					case n.grip.Shortcut(0, ui.KeyHome):
						set(lo)
						state.text = format()
					case n.grip.Shortcut(0, ui.KeyEnd):
						set(hi)
						state.text = format()
					}
				}
				state.held = held && !n.grip.IsDisabled()
				n.grip.Range(lo, hi, *value).Step(step)
			}
		})
		// Overlay the stepper inside the editor's border. The editor owns the
		// shared surface, focus border and intrinsic height.
		if !opts.HideStepper {
			ui.Row(c).Absolute().Top(1).Bottom(1).Right(1).Width(t.Space(3.5) + 1).AlignItems(ui.Stretch).Children(func() {
				ui.Box(c).Width(1).Shrink(0).Background(t.Border)
				ui.Column(c).FillWidth().Children(func() {
					button := func(up bool) ui.Element {
						b := ui.ButtonBase(c).FillWidth().Height(0).Grow(1).Padding(0)
						// Only the outer corners are rounded; interaction backgrounds
						// meet the central separator with square corners.
						radius := max(float32(0), t.Radius-1)
						if up {
							b.Radius(0, radius, 0, 0)
						} else {
							b.Radius(0, 0, radius, 0)
						}
						b.Disabled((up && *value >= hi) || (!up && *value <= lo))
						if !b.IsDisabled() {
							if b.Pressed() {
								b.Background(t.SurfacePressed)
							} else if b.Hovered() {
								b.Background(t.SurfaceHover)
							}
						}
						b.DrawOver(func(p *ui.Painter, r ui.Rect) {
							icon := lucide.ChevronDown()
							if up {
								icon = lucide.ChevronUp()
							}
							size := t.Space(2.5)
							p.Icon(icon, ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, t.Text)
						})
						return dimmed(b)
					}
					n.increase = button(true)
					ui.Box(c).FillWidth().Height(1).Background(t.Border)
					n.decrease = button(false)
					if n.increase.Clicked() {
						advance(step)
					}
					if n.decrease.Clicked() {
						advance(-step)
					}
				})
			})
		}
		n.input.Range(lo, hi, *value).Step(step)
	})
	state.value = *value
	n.Label("")
	return n
}
