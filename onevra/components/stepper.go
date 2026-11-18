package components

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// Stepper motion: a jump across several steps fills or drains its connectors
// one after another, then the current step's ring grows out of its marker.
const (
	stepperStagger   = 80 * time.Millisecond
	stepperRingDelay = 120 * time.Millisecond
	stepperFast      = 160 * time.Millisecond
	stepperStandard  = 240 * time.Millisecond
)

var (
	stepperEnter    = ui.CubicBezier(0.16, 1, 0.3, 1)
	stepperStandOut = ui.CubicBezier(0.22, 1, 0.36, 1)
	// stepperSmooth fills connectors and resizes changing text without overshoot.
	stepperSmooth = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}
	// stepperSnappy pops glyphs and rings with a hint of bounce.
	stepperSnappy = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.12}}
	stepperLeave  = ui.Animation{Duration: stepperFast, Ease: stepperStandOut}
	stepperColor  = ui.Animation{Duration: stepperStandard, Ease: stepperStandOut}
	stepperText   = ui.ElementTransition{
		Duration: stepperStandard, Ease: stepperEnter, Colors: true,
		Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{Y: -4},
	}
	stepperSlot = ui.ElementTransition{Size: true, Animation: &stepperSmooth}
)

// StepperStep is one step of a Stepper, identified by its position.
type StepperStep struct {
	Label string
	// Description is a short hint under the label.
	Description string
	// Error marks the step as failed: its marker turns into an alert and the
	// message replaces the description.
	Error string
}

// StepperOptions configure a Stepper.
type StepperOptions struct {
	// Orientation Vertical stacks the steps for a sidebar or a narrow column,
	// with labels beside the markers. Steps run in a row by default.
	Orientation Orientation
	// Selectable lets people return to a completed step by click or keyboard;
	// StepperControl.Chosen reports it. Otherwise the stepper is a read-only
	// indicator.
	Selectable bool
	// CurrentDescription shows only the current step's description, for tight
	// spaces. Errors always show.
	CurrentDescription bool
	// Compact shows markers only, keeping labels for assistive technology. A
	// horizontal compact stepper names the current step underneath. Hosts
	// choose it from the space they give the stepper.
	Compact bool
	// Label names the stepper for assistive technology; Onevra's catalog
	// supplies one when empty.
	Label string
	// CompleteLabel is announced, and shown in the compact caption, once every
	// step is complete; Onevra's catalog supplies one when empty.
	CompleteLabel string
}

// StepperControl retains step choice separately from Element styling.
type StepperControl struct {
	ui.Element
	chosen int
	ok     bool
}

// Chosen reports the completed step chosen in this frame.
func (s *StepperControl) Chosen() (int, bool) { return s.chosen, s.ok }

type stepperStatus int

const (
	stepperUpcoming stepperStatus = iota
	stepperCurrent
	stepperComplete
	stepperError
)

type stepperGlyph int

const (
	stepperNumber stepperGlyph = iota
	stepperCheck
	stepperAlert
)

// stepperTravel remembers where progress came from, so a jump staggers the
// steps it crosses.
type stepperTravel struct {
	to, from int
	built    bool
}

type stepperTravelKey struct{}

type stepperKey struct {
	name string
	i    int
}

// Stepper shows where a person is in a multi-step flow, for onboarding,
// checkout and setup. current is the index of the step in progress; set it
// to len(steps) once every step is done. Out-of-range values clamp.
// Applications own the steps, the flow and its content, placed beside it.
//
// Completed steps show a check, the current step a ring around its number
// and failed steps an alert. With Selectable, completed steps are buttons:
// Tab enters the steps, the arrows move among those already reached, and
// Home and End go to the first and the last of them. Changes of the current
// step are announced. Reduced Motion skips the motion.
func Stepper(c *ui.Context, steps []StepperStep, current int, opts StepperOptions) *StepperControl {
	t := c.Theme()
	words := localized(c)
	count := len(steps)
	active := min(max(current, 0), count)
	label, completeLabel := opts.Label, opts.CompleteLabel
	if label == "" {
		label = words.messages.Text("stepperProgress")
	}
	if completeLabel == "" {
		completeLabel = words.messages.Text("stepperComplete")
	}
	var now *StepperStep
	if active < count {
		now = &steps[active]
	}

	vertical := opts.Orientation == Vertical
	s := &StepperControl{Element: ui.Column(c).MinWidth(0).Role(ui.RoleGroup).Label(label)}
	if !vertical {
		s.Element.FillWidth()
	}
	travel := ui.Local(s.Element, stepperTravelKey{}, func() stepperTravel { return stepperTravel{to: active, from: active} })
	if travel.to != active {
		travel.from, travel.to = travel.to, active
		if travel.built {
			if now != nil {
				c.Announce(words.messages.Format("stepperAnnounce", map[string]any{"step": active + 1, "count": count, "label": now.Label}))
			} else {
				c.Announce(completeLabel)
			}
		}
	}
	travel.built = true
	from := travel.from
	delayAt := func(i int) time.Duration {
		if active > from && i >= from && i < active {
			return time.Duration(i-from) * stepperStagger
		}
		if active < from && i >= active && i < from {
			return time.Duration(from-1-i) * stepperStagger
		}
		return 0
	}
	ringDelay := time.Duration(max(0, active-from-1, from-active-1))*stepperStagger + stepperRingDelay

	marker := t.Space(7)
	halo := t.Space(1)
	markers := make([]ui.Element, count)
	statusText := [...]string{
		stepperUpcoming: words.messages.Text("stepperUpcoming"), stepperCurrent: words.messages.Text("stepperCurrent"),
		stepperComplete: words.messages.Text("stepperCompleted"), stepperError: words.messages.Text("stepperError"),
	}

	s.Element.Children(func() {
		var list ui.Element
		if vertical {
			list = ui.Column(c).FillWidth().MinWidth(0)
		} else {
			list = ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Start)
		}
		if opts.Selectable {
			if vertical {
				list.FocusGroup(ui.Vertical)
			} else {
				list.FocusGroup(ui.Horizontal)
			}
		}
		list.Children(func() {
			for i, step := range steps {
				status := stepperUpcoming
				switch {
				case step.Error != "":
					status = stepperError
				case i < active:
					status = stepperComplete
				case i == active:
					status = stepperCurrent
				}
				glyph := stepperNumber
				if step.Error != "" {
					glyph = stepperAlert
				} else if i < active {
					glyph = stepperCheck
				}
				detail := step.Error
				if detail == "" && (!opts.CurrentDescription || i == active) {
					detail = step.Description
				}
				clickable := opts.Selectable && i < active

				item := ui.Column(c).Key(i).MinWidth(0)
				last := i == count-1
				switch {
				case vertical && !last && opts.Compact:
					item.Padding(0, 0, t.Space(6), 0)
				case vertical && !last:
					item.Padding(0, 0, t.Space(5), 0)
				case !vertical && !last:
					item.Grow(1).Basis(0).PaddingEnd(t.Space(4))
				case !vertical:
					item.Shrink(0).MaxWidthPercent(100)
				}
				item.Children(func() {
					var head ui.Element
					if opts.Selectable {
						head = ui.ButtonBase(c).Disabled(i > active).FocusRing(false)
					} else {
						head = ui.Box(c)
					}
					if vertical {
						head.Row().FillWidth().AlignItems(ui.Start).Gap(t.Space(3))
					} else {
						head.Column().AlignSelf(ui.Start).MaxWidthPercent(100).AlignItems(ui.Start).Gap(t.Space(3))
					}
					head.MinWidth(0).Radius(t.Space(2)).Label(step.Label + ", " + statusText[status])
					if detail != "" {
						head.Description(detail)
					}
					if clickable {
						head.Cursor(ui.CursorPointer)
						if head.Clicked() {
							s.chosen, s.ok = i, true
						}
					}
					hovered := clickable && head.Hovered()
					pressed := clickable && head.Pressed()
					if focus := head.FocusVisible(); focus {
						r := t.Space(2) + t.Space(1)
						head.DrawOver(func(p *ui.Painter, b ui.Rect) {
							o := t.Space(1)
							p.FocusRing(ui.Rect{X: b.X - o, Y: b.Y - o, W: b.W + 2*o, H: b.H + 2*o}, [4]float32{r, r, r, r})
						})
					}
					head.Children(func() {
						markers[i] = stepperMarker(c, i+1, status, glyph, i == active, hovered, pressed, delayAt(i), ringDelay, marker, halo)
						if opts.Compact {
							return
						}
						text := ui.Column(c).MinWidth(0)
						if vertical {
							text.Grow(1).Padding(t.Space(1), 0, 0, 0)
						}
						text.Children(func() {
							fg := t.TextMuted
							switch {
							case status == stepperCurrent || status == stepperError || hovered:
								fg = t.Text
							case status == stepperComplete:
								fg = t.TextMuted.Mix(t.Text, 0.5)
							}
							l := Text(c, step.Label).Wrap().FontWeight(500).Role(ui.RoleNone)
							l.TextColor(l.AnimateColor("color", fg, stepperColor))
							stepperSwapText(c, detail, step.Error != "")
						})
					})
				})
			}
		})
		if count > 1 {
			list.Draw(func(p *ui.Painter, _ ui.Rect) {
				for i := 0; i < count-1; i++ {
					target := float32(0)
					if i < active {
						target = 1
					}
					a := stepperSmooth
					a.Delay = delayAt(i)
					fill := list.AnimateTo(stepperKey{"fill", i}, target, a)
					stepperConnector(p, t, markers[i].LayoutBounds(), markers[i+1].LayoutBounds(), halo, vertical, fill)
				}
			})
		}
		if opts.Compact && !vertical {
			ui.Column(c).FillWidth().MinWidth(0).Margin(t.Space(3), 0, 0, 0).Role(ui.RoleNone).Children(func() {
				caption, detail, failed := completeLabel, "", false
				if now != nil {
					caption, detail, failed = now.Label, now.Description, now.Error != ""
					if failed {
						detail = now.Error
					}
				}
				ui.Column(c).Key("caption").FillWidth().MinWidth(0).Transition(stepperSlot).Children(func() {
					Text(c, caption).Key(caption).Wrap().FontWeight(500).TextColor(t.Text).Transition(stepperText)
				})
				stepperSwapText(c, detail, failed)
			})
		}
	})
	dimmed(s.Element)
	return s
}

// stepperSwapText shows a description or error that rises in as it changes.
// Its slot springs to the new height, so wrapping text eases the steps below
// it down instead of pushing them.
func stepperSwapText(c *ui.Context, text string, failed bool) {
	t := c.Theme()
	ui.Column(c).Key("detail").FillWidth().MinWidth(0).Transition(stepperSlot).Children(func() {
		if text == "" {
			return
		}
		color, kind := t.TextMuted, "description"
		if failed {
			color, kind = t.Danger, "error"
		}
		Text(c, text).Key(kind+":"+text).Wrap().FontSize(t.Rem(.875)).TextColor(color).
			Padding(2, 0, 0, 0).Role(ui.RoleNone).Transition(stepperText)
	})
}

// stepperMarker draws a step's disc and glyph and, on the current step, a
// ring that grows out from behind it. The number, check and alert share one
// spot: the outgoing glyph shrinks away while the next pops in and draws its
// stroke.
func stepperMarker(c *ui.Context, number int, status stepperStatus, glyph stepperGlyph, current, hovered, pressed bool, delay, ringDelay time.Duration, size, halo float32) ui.Element {
	t := c.Theme()
	m := ui.Box(c).Size(size, size).Shrink(0).Role(ui.RoleNone)
	bg, border, fg, ring, width := t.Surface, t.Border.Mix(t.Text, 0.25), t.TextMuted, t.Accent.Alpha(0.18), float32(1)
	switch status {
	case stepperCurrent:
		border, fg, width = t.Accent, t.Text, 1.5
	case stepperComplete:
		bg, border, fg = t.Accent, t.Accent, t.AccentText
		if hovered {
			bg, border = t.AccentHover, t.AccentHover
		}
	case stepperError:
		mix := float32(0.1)
		if hovered {
			mix = 0.18
		}
		bg, border, fg, ring, width = t.Surface.Mix(t.Danger, mix), t.Danger, t.Danger, t.Danger.Alpha(0.16), 1.5
	}
	press := float32(1)
	if pressed {
		press = 0.92
	}
	label := strconv.Itoa(number)
	m.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		ringColor := m.AnimateColor("ring", ring, stepperColor)
		if scale, fade := stepperPop(m, "ring", current, 0.6, ringDelay); fade > 0 {
			d := (size + 2*halo) * scale
			p.Fill(ui.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}, ringColor.Alpha(fade), d/2)
		}
		bg := m.AnimateColor("bg", bg, stepperColor)
		border := m.AnimateColor("border", border, stepperColor)
		fg := m.AnimateColor("fg", fg, stepperColor)
		w := m.AnimateTo("border-width", width, stepperColor)
		s := m.AnimateTo("press", press, stepperSnappy)
		p.Transform(ui.Scaling(s, s), cx, cy, func() {
			p.Fill(r, bg, r.W/2)
			p.Stroke(ui.Rect{X: r.X + w/2, Y: r.Y + w/2, W: r.W - w, H: r.H - w}, border, (r.W-w)/2, w)
			for _, kind := range [...]stepperGlyph{stepperNumber, stepperCheck, stepperAlert} {
				name := [...]string{"number", "check", "alert"}[kind]
				scale, fade := stepperPop(m, name, kind == glyph, 0.5, delay)
				if kind != glyph && fade <= 0 {
					// A stroke that left draws itself again when it comes back.
					m.Jump(stepperKey{name, stepperDraw}, 0)
					m.Jump(stepperKey{name, stepperDot}, 0)
					continue
				}
				color := fg.Alpha(fade)
				p.Transform(ui.Scaling(scale, scale), cx, cy, func() {
					stepperDrawGlyph(p, m, t, kind, name, label, cx, cy, r.W, color, delay)
				})
			}
		})
	})
	return m
}

// Animated parts of a marker, keyed with the glyph or ring they belong to.
const (
	stepperScale = iota
	stepperFade
	stepperDraw
	stepperDot
)

// stepperPop returns the scale and opacity of a part that pops in after delay
// while on, and shrinks away at once when off.
func stepperPop(m ui.Element, name string, on bool, rest float32, delay time.Duration) (scale, fade float32) {
	if !on {
		return m.AnimateTo(stepperKey{name, stepperScale}, rest, stepperLeave), m.AnimateTo(stepperKey{name, stepperFade}, 0, stepperLeave)
	}
	pop := stepperSnappy
	pop.Delay = delay
	return m.AnimateTo(stepperKey{name, stepperScale}, 1, pop), m.AnimateTo(stepperKey{name, stepperFade}, 1, ui.Animation{Duration: stepperFast, Delay: delay})
}

// stepperDrawGlyph draws a number, or a check or an alert on a 24-unit grid
// half the disc wide whose stroke draws itself in.
func stepperDrawGlyph(p *ui.Painter, m ui.Element, t *ui.Theme, kind stepperGlyph, name, label string, cx, cy, size float32, color ui.Color, delay time.Duration) {
	if kind == stepperNumber {
		span := ui.Span{Text: label, Size: t.Rem(.85), Weight: 500, Features: "tnum", Color: color}
		w, h := p.MeasureText(0, span)
		p.RichText(cx-w/2, cy-h/2, 0, span)
		return
	}
	trim := m.AnimateTo(stepperKey{name, stepperDraw}, 1, ui.Animation{Duration: stepperStandard, Ease: stepperEnter, Delay: delay + 40*time.Millisecond})
	unit := size / 2 / 24
	at := func(x, y float32) (float32, float32) { return cx + (x-12)*unit, cy + (y-12)*unit }
	var path ui.Path
	if kind == stepperCheck {
		path.MoveTo(at(5.5, 12.5))
		path.LineTo(at(9.75, 16.75))
		path.LineTo(at(18.5, 8))
	} else {
		path.MoveTo(at(12, 6.75))
		path.LineTo(at(12, 13.25))
	}
	if trim > 0 {
		p.StrokePath(path.Trim(0, trim), 2.5*unit, color)
	}
	if kind == stepperAlert {
		dot := ui.Animation{Spring: stepperSnappy.Spring, Delay: delay + stepperFast}
		if s := m.AnimateTo(stepperKey{name, stepperDot}, 1, dot); s > 0 {
			x, y := at(12, 17.4)
			r := 1.4 * unit * s
			p.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, color, r)
		}
	}
}

// stepperConnector draws the track between two markers and the accent fill
// along it from the earlier one. Both ends keep a clear gap from the markers
// and the current step's ring; right-to-left rows fill from the right.
func stepperConnector(p *ui.Painter, t *ui.Theme, a, b ui.Rect, halo float32, vertical bool, fill float32) {
	const thickness = 2
	var track, filled ui.Rect
	if vertical {
		gap := halo + 2
		y0, y1 := a.Y+a.H+gap, b.Y-gap
		if y1 <= y0 {
			return
		}
		x := a.X + a.W/2 - thickness/2
		track = ui.Rect{X: x, Y: y0, W: thickness, H: y1 - y0}
		filled = ui.Rect{X: x, Y: y0, W: thickness, H: (y1 - y0) * fill}
	} else {
		gap := halo + t.Space(1)
		y := a.Y + a.H/2 - thickness/2
		if b.X >= a.X {
			x0, x1 := a.X+a.W+gap, b.X-gap
			if x1 <= x0 {
				return
			}
			track = ui.Rect{X: x0, Y: y, W: x1 - x0, H: thickness}
			filled = ui.Rect{X: x0, Y: y, W: (x1 - x0) * fill, H: thickness}
		} else {
			x0, x1 := b.X+b.W+gap, a.X-gap
			if x1 <= x0 {
				return
			}
			w := (x1 - x0) * fill
			track = ui.Rect{X: x0, Y: y, W: x1 - x0, H: thickness}
			filled = ui.Rect{X: x1 - w, Y: y, W: w, H: thickness}
		}
	}
	p.Fill(track, t.Border, thickness/2)
	if fill > 0 {
		p.Clip(track, thickness/2, func() { p.Fill(filled, t.Accent, thickness/2) })
	}
}
