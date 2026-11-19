package components

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// CarouselState is the application-owned state of a Carousel.
type CarouselState struct {
	// Index is the slide in the center. The carousel sets it as the user
	// browses; setting it glides the slides there.
	Index int
	// Playing rotates through the slides every CarouselOptions.Interval.
	// The play control toggles it; set it to start rotating on show.
	Playing bool
}

// CarouselOptions configures a Carousel.
type CarouselOptions struct {
	// Label names the carousel for assistive technology, such as "Harbour
	// suites".
	Label string
	// Count is the number of slides, and Slide builds slide i to fill the
	// slide's width. Slides share the height of the tallest. Every slide stays
	// built, so keep the set short.
	Count int
	Slide func(c *ui.Context, i int)
	// SlideWidth is the widest a slide grows, in DIPs: 340 scaled to the
	// theme's spacing when zero. Slides take at most 80% of the carousel's
	// width, so their neighbors stay in view.
	SlideWidth float32
	// SlideLabel names slide i of count and its indicator: "2 of 5" from
	// Onevra's catalog when nil.
	SlideLabel func(i, count int) string
	// Interval, when positive, adds a play control that rotates to the next
	// slide every Interval while CarouselState.Playing. Rotation pauses on
	// hover, keyboard focus, dragging and off screen, and is unavailable
	// with Reduced Motion.
	Interval time.Duration
}

const (
	// Design DIPs, scaled to the theme's spacing: the widest slide, the
	// carousel width under which its controls tighten, a resting indicator
	// dot and the active pill, and the round controls.
	carouselSlideWidth = 340
	carouselNarrow     = 340
	carouselDot        = 6
	carouselPill       = 22
	carouselControl    = 32
	// carouselShare is the most of the carousel's width a slide takes.
	carouselShare = .8
	// carouselScale and carouselDim are the scale and opacity of a slide
	// one full step from the center.
	carouselScale = .9
	carouselDim   = .5
	// carouselFlick is the release speed, in design DIPs per second, that
	// always moves at least one slide.
	carouselFlick = 360
	// carouselPaging is the share of a flick's speed kept each millisecond.
	carouselPaging = .995
)

var (
	carouselGlide = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}
	// carouselJump moves the track within a frame: under a swipe, and to
	// keep the centered slide when the carousel resizes.
	carouselJump = ui.Animation{Duration: time.Nanosecond}
	// carouselSettle is how long a horizontal swipe must stop before the
	// track settles on the nearest slide.
	carouselSettle = 120 * time.Millisecond
	carouselRefill = 240 * time.Millisecond
)

// carouselKey keys the retained record of a Carousel.
type carouselKey struct{}

// carouselDrainKey keys the animation of the active indicator's drain.
type carouselDrainKey struct{}

// carouselRecord is what a Carousel keeps from frame to frame.
type carouselRecord struct {
	// offset is the track's resting offset, which Draggable moves and
	// glides to; step the distance between slide centers it was laid out
	// with, and shown the offset the last frame showed.
	offset      ui.Point
	step, shown float32
	// index is the last Index seen, to notice the application moving it.
	index int
	// from is the slide a drag started on, and velocity its pointer's speed
	// in the last frame, for flicks.
	dragging bool
	from     int
	velocity float32
	// A horizontal swipe moves the track to raw, until it stops at swiped.
	swiping bool
	raw     float32
	swiped  time.Time
	// elapsed is how long the active slide has rotated, counted from tick.
	elapsed time.Duration
	tick    time.Time
	// paused holds rotation for keyboard focus inside the carousel, as the
	// last frame found it.
	paused bool
}

// Carousel shows Count slides in a centered row, with neighbors partly in
// view as a hint that there is more. The active slide sits in the center;
// the others shrink and dim with their distance from it, frame by frame as
// the track moves. Below the slides, a row of indicators stretches the active
// one into a pill that travels with the slides, beside previous and next
// arrows and, with an Interval, a play control whose pill drains to show the
// time left on the slide.
//
// Dragging the track follows the pointer and settles with momentum, rubber
// banding past the ends; a flick moves at least one slide, a horizontal
// touchpad swipe moves it as well, and a click on a neighbor centers it. The
// arrows, Home and End move the slides while the slide area has the focus,
// and the indicators are tabs. Presses on interactive slide content stay
// with that content. Inactive slides are disabled, so their controls take
// neither focus nor clicks. Slide changes the user makes are announced.
// Reduced Motion moves without gliding.
//
// Applications own the state, slide content and labels. Changed on the
// returned element reports an Index the user chose in this frame.
func Carousel(c *ui.Context, state *CarouselState, options CarouselOptions) ui.Element {
	t := c.Theme()
	s := t.Space(.25)
	n := max(options.Count, 0)
	last := max(n-1, 0)
	state.Index = min(max(state.Index, 0), last)
	m := l10n.In(c.Locale())
	label := options.SlideLabel
	if label == nil {
		label = func(i, count int) string {
			return m.Format("slideOf", map[string]any{"index": i + 1, "count": count})
		}
	}
	root := ui.Column(c).FillWidth().MinWidth(0).Gap(t.Space(4)).Role(ui.RoleGroup).Label(options.Label)
	r := ui.Local(root, carouselKey{}, func() carouselRecord { return carouselRecord{index: state.Index} })
	now := c.Now()
	reduce := c.Preferences().ReduceMotion

	// The width is the last frame's layout, which the first frame lacks: it
	// hides the slides rather than show them off center, and builds again.
	width := float32(400) * s
	if b := root.LayoutBounds(); b.W > 0 {
		width = b.W
	} else {
		root.Invisible()
		c.AnimationFrame()
	}
	widest := options.SlideWidth
	if widest <= 0 {
		widest = carouselSlideWidth * s
	}
	slideW := max(min(width*carouselShare, widest), 1)
	gap := t.Space(4)
	step := slideW + gap
	pad := (width - slideW) / 2
	narrow := width < carouselNarrow*s

	choose := func(i int, announce bool) {
		i = min(max(i, 0), last)
		r.offset.X = -float32(i) * step
		if i == state.Index {
			return
		}
		state.Index, r.index, r.elapsed = i, i, 0
		root.SetChanged()
		if announce {
			c.Announce(label(i, n))
		}
		c.Invalidate()
	}

	animation := carouselGlide
	if state.Index != r.index && !r.dragging {
		// The application moved the slides.
		r.index, r.elapsed = state.Index, 0
		r.offset.X = -float32(state.Index) * step
	}
	if r.step != step && !r.dragging {
		// A resize keeps the active slide centered at once.
		if r.step > 0 {
			animation = carouselJump
		}
		r.step, r.offset.X = step, -float32(state.Index)*step
	}
	if r.swiping {
		animation = carouselJump
		r.offset.X = r.raw
		if wait := carouselSettle - now.Sub(r.swiped); wait > 0 {
			c.After(wait)
		} else {
			r.swiping = false
			animation = carouselGlide
			choose(int(math.Round(float64(-r.raw/step))), true)
		}
	}

	snap := func(p ui.Point) ui.Point {
		// MyGo projects a throw as a free scroll; a flick here carries only as
		// far as a paging deceleration would, from where the track showed.
		if r.dragging {
			p.X = r.shown + r.velocity*carouselPaging/(1-carouselPaging)/1000
		}
		i := int(math.Round(float64(-p.X / step)))
		if v := r.velocity; v < -carouselFlick*s && i <= r.from {
			i = r.from + 1
		} else if v > carouselFlick*s && i >= r.from {
			i = r.from - 1
		}
		return ui.Point{X: -float32(min(max(i, 0), last)) * step}
	}

	var viewport, track ui.Element
	var tabFocus, arrowFocus bool
	var play ui.Element
	root.Children(func() {
		viewport = ui.Box(c).FillWidth().MinWidth(0).ClipX().Focusable().FocusRing(false).Role(ui.RoleGroup).Label(m.Text("slides"))
		if n > 1 {
			switch {
			case viewport.Shortcut(0, ui.KeyLeft):
				choose(state.Index-1, true)
			case viewport.Shortcut(0, ui.KeyRight):
				choose(state.Index+1, true)
			case viewport.Shortcut(0, ui.KeyHome):
				choose(0, true)
			case viewport.Shortcut(0, ui.KeyEnd):
				choose(last, true)
			}
			viewport.HandleInput(func(ev ui.InputEvent) bool {
				// A horizontal touchpad swipe moves the track one to one; a
				// vertical one scrolls the page.
				if ev.Kind != ui.InputScroll || r.dragging || (!r.swiping && abs32(ev.DX) <= abs32(ev.DY)) {
					return false
				}
				if !r.swiping {
					r.swiping, r.raw = true, r.shown
				}
				r.raw = min(max(r.raw-ev.DX, -float32(last)*step), 0)
				r.swiped = ev.Time
				return true
			})
		}
		focus := viewport.FocusVisible()
		viewport.DrawOver(func(p *ui.Painter, rect ui.Rect) {
			if focus {
				p.FocusRing(rect, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
			}
		})
		viewport.Children(func() {
			track = ui.Row(c).Width(2*pad + float32(n)*slideW + float32(last)*gap).Shrink(0).PaddingX(pad).Gap(gap)
			if n > 1 {
				track.Draggable(&r.offset, ui.DraggableOptions{
					Axis: ui.Horizontal, Bounds: &ui.Rect{X: -float32(last) * step, W: float32(last) * step},
					Snap: snap, Animation: animation,
				})
				if track.Dragging() {
					if !r.dragging {
						r.dragging, r.from, r.swiping = true, state.Index, false
					}
					r.velocity, _ = track.DragVelocity()
					track.Cursor(ui.CursorGrabbing)
				} else {
					r.dragging, r.velocity = false, 0
				}
				x, _, _ := track.PointerPosition()
				if track.Clicked() {
					// A click on a neighbor centers it.
					if i := int(math.Floor(float64((x - pad + gap/2) / step))); i >= 0 && i < n && i != state.Index {
						choose(i, true)
					}
				}
				r.shown = track.DragOffset().X
				if !r.dragging && !r.swiping {
					// A drag let go, or a press catching a moving track, rests
					// on a slide.
					if at := min(max(int(math.Round(float64(-r.offset.X/step))), 0), last); at != state.Index {
						choose(at, true)
					}
				}
			} else {
				r.shown = 0
			}
			position := -r.shown / step
			track.Children(func() {
				for i := range n {
					e := 1 - min(abs32(position-float32(i)), 1)
					scale := carouselScale + (1-carouselScale)*e
					slide := ui.Column(c).Key(i).Width(slideW).Shrink(0).MinWidth(0).Role(ui.RoleGroup).Label(label(i, n)).
						Scale(scale, scale).Opacity(carouselDim + (1-carouselDim)*e).Disabled(i != state.Index)
					if options.Slide != nil {
						slide.Children(func() { options.Slide(c, i) })
					}
				}
			})
		})
		if n < 2 {
			return
		}
		position := min(max(-r.shown/step, 0), float32(last))
		controls := ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(2)).PaddingX(pad)
		if narrow {
			controls.Gap(t.Space(1)).PaddingX(max(0, pad-t.Space(2)))
		}
		canRotate := options.Interval > 0 && !reduce
		controls.Children(func() {
			if canRotate {
				icon, name := lucide.Play, m.Text("startRotation")
				if state.Playing {
					icon, name = lucide.Pause, m.Text("pauseRotation")
				}
				play = carouselButton(c, icon, name, false)
				if play.Clicked() {
					state.Playing = !state.Playing
					r.elapsed = 0
				}
			}
			// Each indicator grows toward a pill as the track nears its
			// slide, so the pill hands over continuously instead of hopping.
			selected := state.Index
			tabs := ui.TabsBase(c, &selected, n)
			tabs.List.Label(m.Text("chooseSlide")).MinWidth(0)
			drain := float32(0)
			if canRotate {
				drain = min(float32(r.elapsed)/float32(options.Interval), 1)
			}
			tabs.List.Children(func() {
				for i := range n {
					e := max(0, 1-abs32(position-float32(i)))
					w := (carouselDot + (carouselPill-carouselDot)*e) * s
					padding := 5 * s
					if narrow {
						padding = 4 * s
					}
					tab := tabs.Tab(i).Width(w + 2*padding).Height(carouselControl * s).FocusRing(false).Label(label(i, n))
					d := float32(0)
					if i == state.Index {
						if state.Playing && canRotate {
							d = tab.Jump(carouselDrainKey{}, drain)
						} else {
							// Pausing refills the pill.
							d = tab.AnimateWith(carouselDrainKey{}, 0, carouselRefill, ui.EaseOut)
						}
					}
					base := t.TextMuted.Alpha(.45)
					if tab.Hovered() {
						base = t.TextMuted
					}
					focus := tab.FocusVisible()
					tabFocus = tabFocus || focus
					dot := carouselDot * s
					tab.Draw(func(p *ui.Painter, rect ui.Rect) {
						pill := ui.Rect{X: rect.X + (rect.W-w)/2, Y: rect.Y + (rect.H-dot)/2, W: w, H: dot}
						p.Fill(pill, base, dot/2)
						p.Fill(pill, t.Text.Alpha(e), dot/2)
						// The drain covers the fill from the right as the slide's
						// time runs out.
						if d > 0 {
							p.Fill(ui.Rect{X: pill.X + pill.W*(1-d), Y: pill.Y, W: pill.W * d, H: pill.H}, base, dot/2)
						}
					})
					tab.DrawOver(func(p *ui.Painter, rect ui.Rect) {
						if focus {
							in := 4 * s
							inset := ui.Rect{X: rect.X + in, Y: rect.Y + in, W: rect.W - 2*in, H: rect.H - 2*in}
							p.FocusRing(inset, [4]float32{inset.H / 2, inset.H / 2, inset.H / 2, inset.H / 2})
						}
					})
				}
			})
			if selected != state.Index {
				choose(selected, true)
			}
			ui.Box(c).Grow(1)
			arrows := ui.Row(c).Gap(t.Space(2)).Shrink(0)
			if narrow {
				arrows.Gap(t.Space(1))
			}
			arrows.Children(func() {
				prev := carouselButton(c, lucide.ChevronLeft, m.Text("previousSlide"), state.Index == 0)
				next := carouselButton(c, lucide.ChevronRight, m.Text("nextSlide"), state.Index == last)
				arrowFocus = prev.FocusVisible() || next.FocusVisible()
				// The arrows stay focusable at the ends, so the focus is not lost.
				if prev.Clicked() && state.Index > 0 {
					choose(state.Index-1, true)
				}
				if next.Clicked() && state.Index < last {
					choose(state.Index+1, true)
				}
			})
		})

		// Rotation counts the active slide's time while nothing holds it.
		r.paused = viewport.FocusVisible() || tabFocus || arrowFocus
		running := canRotate && state.Playing && !root.Hovered() && !r.paused && !r.dragging && !r.swiping && root.InView()
		if !running {
			r.tick = time.Time{}
			return
		}
		if !r.tick.IsZero() {
			r.elapsed += now.Sub(r.tick)
		}
		r.tick = now
		if r.elapsed >= options.Interval {
			next := state.Index + 1
			if next > last {
				next = 0
			}
			choose(next, false)
			r.elapsed = 0
		}
		c.AnimationFrame()
	})
	return root
}

// carouselButton is a round, bordered control of a Carousel. A dimmed one
// stays focusable but looks unavailable, for an arrow at the end.
func carouselButton(c *ui.Context, icon func() *ui.SVG, label string, dimmed bool) ui.Element {
	t := c.Theme()
	size := max(carouselControl*t.Space(.25), TouchMin(c))
	b := ui.ButtonBase(c).Size(size, size).Shrink(0).Radius(size/2).Border(1, t.Border).Label(label).TextColor(t.Text)
	face := t.Surface
	switch {
	case dimmed:
		b.Opacity(.5).TextColor(t.TextMuted)
	case b.Pressed():
		face = t.SurfacePressed
		b.Scale(.97, .97)
	case b.Hovered():
		face = t.SurfaceHover
	}
	b.Background(face).Center().Children(func() {
		ui.Box(c).Key(label).Center().Transition(ui.ElementTransition{
			Duration: 160 * time.Millisecond,
			Enter:    &ui.Motion{Scale: .6}, Exit: &ui.Motion{Scale: .6},
		}).Children(func() {
			Icon(c, icon).Size(t.Space(4), t.Space(4))
		})
	})
	Tooltip(c, b, label)
	return b
}

func abs32(v float32) float32 { return float32(math.Abs(float64(v))) }
