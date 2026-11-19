// Package shaders provides ambient GPU backgrounds for welcome and empty
// pages: quiet animated fields that follow the pointer and ripple where it
// clicks, behind application content.
//
// Each background is an element that fills its parent; build the page inside
// it with Children. The backgrounds draw only on a GPU renderer
// (ui.Painter.Accelerated), where a full-window shader costs little; on the
// CPU renderer, including ui.Tester, they leave the theme background as it
// is. Reduce Motion freezes their drift and skips ripples; the pointer glow
// remains, without its lag.
package shaders

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
)

// Options configure a background. Zero values take the defaults.
type Options struct {
	// Color is the resting color of the pattern; zero takes the theme's
	// muted text.
	Color ui.Color
	// Accent colors what the pointer and ripples touch; zero takes the
	// theme accent.
	Accent ui.Color
	// Intensity scales the contrast of the pattern; zero means 1. Values
	// up to 2 stay usable behind text.
	Intensity float32
	// Scale scales the pattern, such as SignalField's dot spacing; zero
	// means 1.
	Scale float32
	// Still freezes the ambient drift while pointer feedback continues.
	Still bool
	// Inert ignores the pointer: no glow and no ripples.
	Inert bool
}

// MaxRipples is how many click ripples a background shows at once; a new
// click replaces the oldest.
const MaxRipples = 6

// rippleLife is how long a ripple stays visible.
const rippleLife = 1400 * time.Millisecond

// timeWrap keeps the time uniform small enough for float32 precision; the
// drift is slow, so the jump once in this period goes unnoticed.
const timeWrap = 3600.0

// driftFrame is how often the ambient drift moves on. It is slow, and each
// of its frames paints the whole window again, so following every refresh
// of the display would cost much for nothing to see. Ripples spread fast,
// and move with the display.
const driftFrame = 50 * time.Millisecond

type ripple struct {
	x, y float32
	at   time.Time
}

// stateKey keys a background's state on its element.
type stateKey struct{}

// state is what a background keeps between frames, on its element.
type state struct {
	start         time.Time
	ripples       [MaxRipples]ripple
	next          int
	lastX, lastY  float32
	seenPointer   bool
	rippling      bool
	values        *shader.Values
	rippleUniform [MaxRipples * 4]float32
}

// press records a ripple at x, y.
func (s *state) press(x, y float32, at time.Time) {
	s.ripples[s.next] = ripple{x: x, y: y, at: at}
	s.next = (s.next + 1) % MaxRipples
}

// liveRipples fills the ripple uniform for now: x, y, age in seconds and
// strength, 0 for slots without a live ripple. It reports how many live.
func (s *state) liveRipples(now time.Time) int {
	live := 0
	for i, r := range s.ripples {
		age := now.Sub(r.at)
		strength := float32(0)
		if !r.at.IsZero() && age >= 0 && age < rippleLife {
			strength = 1
			live++
		}
		s.rippleUniform[i*4+0] = r.x
		s.rippleUniform[i*4+1] = r.y
		s.rippleUniform[i*4+2] = float32(age.Seconds())
		s.rippleUniform[i*4+3] = strength
	}
	return live
}

// effect is one background's compiled shader and its uniforms.
type effect struct {
	key                                                 string
	shader                                              *shader.Shader
	time, pointer, ripples, color, accent, level, scale shader.Uniform
	defaultColor                                        func(ui.Theme) ui.Color
}

// Ripple ripples background, an element a background function returned, where
// the pointer last was over it, as a click there does. Content that takes the
// pointer's input itself, such as components.Honeycomb with its OnPress,
// hides its presses from the background, and calls Ripple for them. It does
// nothing where the background skips ripples.
func Ripple(c *ui.Context, background ui.Element) {
	s := ui.Local(background, stateKey{}, func() state { return state{} })
	if s == nil || !s.rippling || !s.seenPointer {
		return
	}
	s.press(s.lastX, s.lastY, c.Now())
}

func newEffect(key, source string, defaultColor func(ui.Theme) ui.Color) *effect {
	sh := shader.MustCompile(source)
	return &effect{
		key: key, shader: sh, defaultColor: defaultColor,
		time: sh.Uniform("Time"), pointer: sh.Uniform("Pointer"), ripples: sh.Uniform("Ripples"),
		color: sh.Uniform("Tint"), accent: sh.Uniform("Accent"), level: sh.Uniform("Intensity"), scale: sh.Uniform("Scale"),
	}
}

func (fx *effect) build(c *ui.Context, o Options) ui.Element {
	t := c.Theme()
	root := ui.Box(c).Key(fx.key).Fill().MinWidth(0).MinHeight(0).Clip()
	s := ui.Local(root, stateKey{}, func() state { return state{values: fx.shader.NewValues()} })
	reduce := c.Preferences().ReduceMotion

	color, accent := o.Color, o.Accent
	if color == (ui.Color{}) {
		color = fx.defaultColor(*t)
	}
	if accent == (ui.Color{}) {
		accent = t.Accent
	}
	level, scale := o.Intensity, o.Scale
	if level <= 0 {
		level = 1
	}
	if scale <= 0 {
		scale = 1
	}

	var px, py, presence float32
	s.rippling = !o.Inert && !reduce
	if !o.Inert {
		x, y, over := root.PointerPosition()
		if over {
			s.lastX, s.lastY = x, y
			if !s.seenPointer {
				// The glow appears where the pointer enters rather than
				// gliding in from the corner.
				s.seenPointer = true
				root.Jump(fx.key+".x", x)
				root.Jump(fx.key+".y", y)
			}
		}
		spring := ui.SpringOptions{Duration: 450 * time.Millisecond}
		px, py = s.lastX, s.lastY
		if !reduce {
			px = root.Spring(fx.key+".x", s.lastX, spring)
			py = root.Spring(fx.key+".y", s.lastY, spring)
		}
		target := float32(0)
		if over {
			target = 1
		}
		presence = root.Animate(fx.key+".presence", target, 600*time.Millisecond)
		if s.rippling {
			// Clicks on content pass through: the handler only watches.
			// Content taking its own input hides its presses from it, so
			// such content reports them with Ripple.
			root.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind == ui.InputPointerDown && ev.Button == 0 {
					s.press(ev.X, ev.Y, ev.Time)
				}
				return false
			})
		}
	}

	return root.Draw(func(p *ui.Painter, r ui.Rect) {
		if !p.Accelerated() {
			return
		}
		now := p.Now()
		if s.start.IsZero() {
			s.start = now
		}
		elapsed, drifting := float32(0), !o.Still && !reduce
		if drifting {
			elapsed = float32(math.Mod(now.Sub(s.start).Seconds(), timeWrap))
		}
		live := s.liveRipples(now)
		s.values.
			Float(fx.time, elapsed).
			Vec3(fx.pointer, px, py, presence).
			Floats(fx.ripples, s.rippleUniform[:]...).
			Color(fx.color, color).
			Color(fx.accent, accent).
			Float(fx.level, level).
			Float(fx.scale, scale)
		fx.shader.Draw(p, r, shader.Options{Values: s.values})
		switch {
		case live > 0:
			p.AnimationFrame()
		case drifting:
			p.After(driftFrame)
		}
	})
}
