package components

import (
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// HoldToConfirmTone selects the color a hold fills with.
type HoldToConfirmTone int

const (
	// HoldAccent fills with the theme accent.
	HoldAccent HoldToConfirmTone = iota
	// HoldDanger tints the face and fills with the danger color, for an
	// irreversible action.
	HoldDanger
	// HoldNeutral fills with the text color.
	HoldNeutral
)

// HoldToConfirmOptions customizes a hold; the zero value is a 1.2-second
// accent hold with a trash icon and the catalog's done label.
type HoldToConfirmOptions struct {
	// ConfirmedLabel replaces the label once the hold completes, such as
	// "Deleted"; empty uses the catalog's "Done".
	ConfirmedLabel string
	// Duration is how long the hold takes; zero is 1.2 seconds.
	Duration time.Duration
	// Icon leads the label until the hold completes; nil uses lucide.Trash.
	Icon *ui.SVG
	Tone HoldToConfirmTone
}

// HoldToConfirmControl exposes the button element. Keep this handle to query
// Changed and Holding after applying Element methods, which return ui.Element.
type HoldToConfirmControl struct {
	ui.Element
	changed, holding bool
}

// Changed reports the frame in which a hold completed and set *confirmed.
func (h *HoldToConfirmControl) Changed() bool { return h.changed }

// Holding reports a hold in progress, for nearby hints such as "Keep holding".
func (h *HoldToConfirmControl) Holding() bool { return h.holding }

// Motion follows the reference component's tokens: a linear fill, a smooth
// rewind, a snappy press and a slightly bouncy width morph.
var (
	holdSmooth = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}
	holdSnappy = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.12}}
	holdMorph  = ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: 0.16}}
	holdEnter  = ui.CubicBezier(0.16, 1, 0.3, 1)
	holdText   = ui.Animation{Duration: 240 * time.Millisecond, Ease: holdEnter}
	// holdResize springs the label frame to the width of new text, and its
	// ancestors with it, which would otherwise take the new width at once.
	holdResize   = ui.ElementTransition{Size: true, Animation: &holdMorph}
	holdSlack    = float32(24)
	holdDuration = 1200 * time.Millisecond
)

type holdSource uint8

const (
	holdIdle holdSource = iota
	holdPointer
	holdKey
)

// holdState is the element's own state between frames; input handlers
// update it on the UI thread before the next build.
type holdState struct {
	source holdSource
	// start and from are when the current hold began and the fill it began
	// from, so a quick re-press resumes rather than starting over.
	start time.Time
	from  float32
	// shown is the fill the last frame showed; released marks a hold ended
	// since then, whose rewind starts at rest.
	shown    float32
	released bool
	w, h     float32
}

func (s *holdState) begin(source holdSource, at time.Time) {
	if s.source == holdIdle {
		s.source, s.start, s.from = source, at, s.shown
	}
}

func (s *holdState) release(source holdSource) {
	if s.source == source {
		s.source, s.released = holdIdle, true
	}
}

// HoldToConfirm creates a button that sets *confirmed only after it is held,
// for destructive or hard-to-undo actions a stray click must not trigger. A
// fill tracks the hold linearly from the leading edge; letting go early, or
// sliding well off the button, rewinds it, and completing morphs the label
// and icon into a done state. Holding Space or Enter works too; moving the
// focus away cancels. Set *confirmed back to false to reset the button.
//
//	h := HoldToConfirm(c, &deleted, "Hold to delete", HoldToConfirmOptions{ConfirmedLabel: "Deleted", Tone: HoldDanger})
//	if h.Changed() { /* delete */ }
//
// Prefer AlertDialog when people need to read the consequences first. The
// label is the accessible name; a localized description explains the hold,
// and completion is announced. Applications own both labels. Key a parent
// container, or pass c.Key, for stable identity: the hold is element state.
func HoldToConfirm(c *ui.Context, confirmed *bool, label string, options ...HoldToConfirmOptions) *HoldToConfirmControl {
	if len(options) > 1 {
		panic("components.HoldToConfirm: at most one options value")
	}
	var o HoldToConfirmOptions
	if len(options) > 0 {
		o = options[0]
	}
	words := localized(c)
	if o.ConfirmedLabel == "" {
		o.ConfirmedLabel = words.HoldDone
	}
	if o.Duration <= 0 {
		o.Duration = holdDuration
	}
	if o.Icon == nil {
		o.Icon = lucide.Trash()
	}
	t := c.Theme()
	b := ui.ButtonBase(c)
	h := &HoldToConfirmControl{Element: b}
	s := ui.Local(b, "onevra.hold", func() holdState { return holdState{} })
	disabled := b.IsDisabled()
	b.HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			// Taking the press keeps the base from reporting a click.
			if !*confirmed && !disabled {
				s.begin(holdPointer, ev.Time)
			}
			return true
		case ui.InputPointerMove:
			if s.source == holdPointer && (ev.X < -holdSlack || ev.Y < -holdSlack || ev.X > s.w+holdSlack || ev.Y > s.h+holdSlack) {
				s.release(holdPointer)
			}
		case ui.InputPointerUp:
			s.release(holdPointer)
			return ev.Button == 0
		case ui.InputKeyDown, ui.InputKeyUp:
			if ev.Key != ui.KeySpace && ev.Key != ui.KeyEnter {
				return false
			}
			if ev.Kind == ui.InputKeyUp {
				s.release(holdKey)
			} else if !ev.Repeat && !*confirmed && !disabled {
				s.begin(holdKey, ev.Time)
			}
			return true
		}
		return false
	})
	if bounds := b.LayoutBounds(); bounds.W > 0 {
		s.w, s.h = bounds.W, bounds.H
	}
	if disabled || *confirmed || (s.source == holdKey && !b.Focused()) {
		s.release(s.source)
	}

	var fill float32
	if s.source != holdIdle {
		p := s.from + float32(max(c.Now().Sub(s.start), 0))/float32(o.Duration)
		if p >= 1 {
			s.source, *confirmed, h.changed = holdIdle, true, true
			c.Announce(o.ConfirmedLabel)
			p = 1
		} else {
			c.AnimationFrame()
		}
		fill = b.Jump("fill", p)
	} else {
		if s.released {
			// Jumping in place zeroes the velocity the linear fill left.
			b.Jump("fill", s.shown)
		}
		target := float32(0)
		if *confirmed {
			target = 1
		}
		fill = b.AnimateTo("fill", target, holdSmooth)
	}
	s.shown, s.released, h.holding = fill, false, s.source != holdIdle

	text := label
	if *confirmed {
		text = o.ConfirmedLabel
	} else {
		b.Description(words.holdHint(o.Duration))
	}
	ink, fillColor, onFill, border := t.Text, t.Accent, t.AccentText, t.Border
	switch o.Tone {
	case HoldDanger:
		ink, fillColor, onFill, border = t.Danger, t.Danger, t.Background, t.Border.Mix(t.Danger, 0.32)
	case HoldNeutral:
		fillColor, onFill = t.Text, t.Background
	}
	face := t.Surface
	if !disabled && !*confirmed && b.Hovered() {
		face = t.Surface.Mix(fillColor, 0.06)
	}
	pose := ui.Pose{}
	if h.holding && !c.Preferences().ReduceMotion {
		// Wide buttons press less deeply, so their edges move as far.
		pose.Scale = 0.97
		if s.w > 220 {
			pose.Scale = 0.985
		}
	}
	b.AnimatePose(pose, holdSnappy)
	face = b.AnimateColor("face", face, ui.Animation{Duration: 160 * time.Millisecond})
	b.Label(text).Radius(t.Radius).Padding(t.Space(1.5), t.Space(3.5)).Background(face).Border(1, border).Transition(holdResize)
	b.Children(func() {
		holdFace(c, o.Icon, text, *confirmed, ink).Transition(holdResize)
		if fill <= 0.001 {
			return
		}
		// A second, inverted face shows through the fill, so the text changes
		// color exactly at its edge. Positioned children paint above the
		// first face; the outer box spans the border so the fill covers it.
		ui.Box(c).Absolute().Top(-1).Bottom(-1).Start(-1).End(-1).Children(func() {
			ui.Box(c).Absolute().Top(0).Bottom(0).Start(0).WidthPercent(min(fill, 1) * 100).Clip().Children(func() {
				ui.Row(c).Absolute().Top(0).Bottom(0).Start(0).WidthPercent(100 / min(fill, 1)).
					Radius(t.Radius).Background(fillColor).Justify(ui.Center).AlignItems(ui.Center).
					Children(func() { holdFace(c, o.Icon, text, *confirmed, onFill) })
			})
		})
	})
	dimmed(b)
	return h
}

// holdFace builds the icon and label. The label frame springs to the width of
// new text while the old text rises out and the new rises in.
func holdFace(c *ui.Context, icon *ui.SVG, text string, done bool, color ui.Color) ui.Element {
	t := c.Theme()
	size := t.Rem(1)
	return ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).TextColor(color).Children(func() {
		swap := ui.ElementTransition{Animation: &holdSnappy, Enter: &ui.Motion{Scale: 0.6}, Exit: &ui.Motion{Scale: 0.6}}
		ui.Box(c).Size(size, size).Shrink(0).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			if !done {
				ui.Icon(c.Key("idle"), icon).Size(size, size).Transition(swap)
				return
			}
			check := ui.Box(c.Key("done")).Size(size, size).Transition(swap)
			// The tick draws itself from its short stroke, as a hand writes it.
			drawn := check.Keyframes("draw", []ui.Keyframe{
				{Value: 0},
				{At: 80 * time.Millisecond, Value: 0},
				{At: 400 * time.Millisecond, Value: 1, Ease: holdEnter},
			})
			check.Draw(func(p *ui.Painter, r ui.Rect) {
				u := r.W / 24
				var path ui.Path
				path.MoveTo(r.X+4*u, r.Y+12.5*u).LineTo(r.X+9*u, r.Y+17.5*u).LineTo(r.X+20*u, r.Y+6.5*u).Trim(0, drawn)
				p.StrokePath(&path, 2*u, color)
			})
		})
		// The exiting copy takes no room, so the frame sizes to the new text.
		ui.Box(c.Key("label")).ClipX().Transition(holdResize).Children(func() {
			lift := 0.3 * t.FontSize
			ui.Text(c.Key(text), text).SingleLine().Shrink(0).Transition(ui.ElementTransition{
				Animation: &holdText, Position: true, Enter: &ui.Motion{Y: lift}, Exit: &ui.Motion{Y: -lift},
			})
		})
	})
}
