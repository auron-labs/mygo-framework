package components

import (
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// SkeletonOptions shapes a Skeleton placeholder: Lines of text, 3 when zero
// and clamped to 1–6, after an optional round Avatar. Label names the loading
// status; empty uses "Loading content" from Onevra's catalog.
type SkeletonOptions struct {
	Lines  int
	Avatar bool
	Label  string
}

// skeletonPulse is one dimming beat of a block; each block starts its beat
// skeletonStagger after the one above, so the blocks pulse in a calm wave.
const (
	skeletonPulse   = 1800 * time.Millisecond
	skeletonStagger = 90 * time.Millisecond
)

// Skeleton creates a pulsing placeholder of text lines for content whose
// shape is known. It fills the available width; lines are proportions of it.
// It is a loading status named by options.Label; the blocks themselves are not
// exposed. Only drawing animates. Reduced Motion stops the pulse.
func Skeleton(c *ui.Context, options SkeletonOptions) ui.Element {
	t := c.Theme()
	reduced := c.Preferences().ReduceMotion
	lines := options.Lines
	if lines == 0 {
		lines = 3
	}
	lines = max(1, min(lines, 6))
	label := options.Label
	if label == "" {
		label = l10n.In(c.Locale()).Text("loadingContent")
	}
	e := ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Start).Gap(t.Space(4)).
		Role(ui.RoleStatus).Label(label)
	e.Children(func() {
		index := 0
		if options.Avatar {
			skeletonBlock(c, index, reduced, true).Size(t.Space(9), t.Space(9)).Shrink(0)
			index++
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(3)).Padding(t.Space(1), 0, 0, 0).Children(func() {
			for i := range lines {
				width, height := float32(100), t.Space(3)
				if i == 0 {
					width, height = 48, t.Space(4)
				}
				if i == lines-1 {
					width = 72
				}
				skeletonBlock(c, index+i, reduced, false).WidthPercent(width).Height(height)
			}
		})
	})
	return e
}

// SkeletonContent shows a Skeleton while loading and content once it is not.
// The placeholder fades out as the content fades in, rising into place, and
// the container's height springs between them; later size changes of the
// content follow with the same spring. Reduced Motion swaps with fades and no
// movement. The application owns loading and builds content synchronously.
func SkeletonContent(c *ui.Context, loading bool, options SkeletonOptions, content func()) ui.Element {
	e := ui.Column(c).FillWidth().MinWidth(0).Transition(ui.ElementTransition{Size: true, Animation: &ui.Smooth})
	// Clip only while the height moves, so focus rings stay visible at rest.
	if e.Transitioning() {
		e.ClipY()
	}
	e.Children(func() {
		if loading {
			Skeleton(c.Key("onevra.skeleton.placeholder"), options).Transition(ui.ElementTransition{
				Colors: true, Duration: 150 * time.Millisecond, Ease: ui.EaseInOut,
				Enter: &ui.Motion{}, Exit: &ui.Motion{},
			})
			return
		}
		ui.Column(c.Key("onevra.skeleton.content")).FillWidth().MinWidth(0).Transition(ui.ElementTransition{
			Colors: true, Duration: 250 * time.Millisecond, Ease: ui.EaseOut,
			Enter: &ui.Motion{Y: 4}, Exit: &ui.Motion{},
		}).Children(content)
	})
	return e
}

// skeletonBlock draws one placeholder shape: a rounded line, or a circle for
// the avatar. Its pulse follows the clock, so skeletons on screen beat together.
func skeletonBlock(c *ui.Context, index int, reduced, round bool) ui.Element {
	t := c.Theme()
	return ui.Box(c).Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 || r.H <= 0 {
			return
		}
		radius := t.Radius
		if round {
			radius = min(r.W, r.H) / 2
		}
		opacity := float32(1)
		if !reduced {
			p.AnimationFrame()
			opacity = skeletonOpacity(p.Now(), index)
		}
		p.Fill(r, t.Border.Alpha(opacity), radius)
	})
}

// skeletonOpacity dims a block from opaque to about half and back over one
// beat, easing in and out of each half, as uiarc's skeleton pulse does.
func skeletonOpacity(now time.Time, index int) float32 {
	elapsed := now.UnixMilli() - (time.Duration(index) * skeletonStagger).Milliseconds()
	period := skeletonPulse.Milliseconds()
	phase := float32((elapsed%period+period)%period) / float32(period)
	dim := 1 - 2*phase
	if dim < 0 {
		dim = -dim
	}
	return 1 - .48*ui.EaseInOut(1-dim)
}
