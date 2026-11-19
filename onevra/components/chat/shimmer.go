package chat

import (
	"math"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// shimmerPeriod is how long the highlight takes to cross the text and rest.
const shimmerPeriod = 1800 * time.Millisecond

// Shimmer draws a single line of text of size in color with a highlight
// sweeping across it, marking ongoing work such as a live activity title.
// It keeps its natural width unless the caller lets it shrink, for example
// with Shrink(1).MinWidth(0); text that then does not fit ends in an
// ellipsis. Only drawing animates; the text is shaped once per build.
// Reduced Motion shows plain text.
func Shimmer(c *ui.Context, text string, size float32, color, highlight ui.Color) ui.Element {
	if c.Preferences().ReduceMotion {
		return components.Text(c, text).Shrink(0).SingleLine().Ellipsis("…").FontSize(size).TextColor(color)
	}
	font := ui.Font{Size: size}
	glyphs := ui.Shape(text, font)
	width := advance(glyphs)
	ellipsis := ui.Shape("…", font)
	m := font.Metrics()
	return ui.Box(c).Label(text).Size(width, m.Ascent+m.Descent).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		p.After(33 * time.Millisecond)
		shown, w := glyphs, width
		// Narrower than the text, the glyphs that fit before an ellipsis show.
		if r.W+.5 < width {
			room := r.W - advance(ellipsis)
			n := 0
			for n < len(glyphs) && glyphs[n].X+glyphs[n].Advance <= room {
				n++
			}
			shown = append(glyphs[:n:n], ellipsis...)
			end := advance(glyphs[:n])
			for i := n; i < len(shown); i++ {
				shown[i].X += end
			}
			w = advance(shown)
		}
		// The highlight's center travels from before the first glyph to past
		// the last, so each sweep enters and leaves softly.
		band := max(w*.25, size*1.25)
		phase := float32(p.Now().UnixMilli()%shimmerPeriod.Milliseconds()) / float32(shimmerPeriod.Milliseconds())
		center := -band + phase*(w+3*band)
		baseline := r.Y + m.Ascent
		for i := range shown {
			g := shown[i : i+1]
			d := float64((g[0].X + g[0].Advance/2 - center) / band)
			p.Glyphs(g, r.X, baseline, color.Mix(highlight, float32(math.Exp(-d*d*2))*.75))
		}
	})
}

// advance is the width of a shaped run.
func advance(glyphs []ui.Glyph) float32 {
	if n := len(glyphs); n > 0 {
		return glyphs[n-1].X + glyphs[n-1].Advance
	}
	return 0
}
