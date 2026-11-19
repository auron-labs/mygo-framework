package components

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// SpinnerVariant selects the appearance of an indeterminate activity indicator.
type SpinnerVariant int

const (
	// SpinnerRing uses the same track and rotating arc as ProgressRing.
	SpinnerRing SpinnerVariant = iota
	// SpinnerSpokes uses twelve fading spokes, like MyGo's built-in spinner.
	SpinnerSpokes
)

// Spinner creates an indicator of work of unknown length. Name it with Label
// for accessibility. Both variants default to a theme-scaled diameter and stay
// circular and centered in non-square layouts. Reduced Motion freezes activity.
// Spokes use the theme's muted text color unless overridden with TextColor.
// Unknown variants use SpinnerRing.
func Spinner(c *ui.Context, variant SpinnerVariant) ui.Element {
	if variant != SpinnerSpokes {
		return ProgressRing(c, -1).Shrink(0)
	}
	t := c.Theme()
	// MyGo's icon primitive resolves TextColor and owns rendering; its animation
	// primitive keeps the twelve-step turn within the window's normal lifecycle.
	e := Icon(c, spinnerSpokesIcon).Size(t.Space(8), t.Space(8)).Shrink(0).
		TextColor(t.TextMuted).Role(ui.RoleProgress).Range(0, 1, -1)
	if !c.Preferences().ReduceMotion {
		phase := e.Loop("onevra.spinner", 900*time.Millisecond, ui.Linear)
		e.Rotate(float32(int(phase*12)) * 30)
	}
	return e
}

// Spoke geometry and fading follow MyGo's indicator (MIT; see LICENSE.mygo).
// Parse one reusable mask, rather than preparing an SVG in every build pass.
var spinnerSpokesIcon = sync.OnceValue(func() *ui.SVG {
	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="-50 -50 100 100" fill="none" stroke="currentColor" stroke-width="9">`)
	for i := range 12 {
		age := (12 - i) % 12
		alpha := 1 - float32(age)/12*0.85
		fmt.Fprintf(&svg, `<path d="M0 -22V-46" transform="rotate(%d)" stroke-opacity="%g"/>`, i*30, alpha)
	}
	svg.WriteString(`</svg>`)
	return ui.MustParseSVG([]byte(svg.String()))
})
