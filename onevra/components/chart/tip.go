package chart

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// tipSource remembers whether the pointer or the keyboard made a plot's
// selection last.
type tipSource struct {
	x, y          float32
	over, pointer bool
	index         int
}

// tip shows the selection in a tooltip beside the pointer while the pointer
// chose it over area, or beside the element mark places in area while the
// keyboard chose it and area has the focus. Call it among area's children,
// after the plot's selection handling.
func tip(c *ui.Context, area ui.Element, opts Options, selection Selection, mark func() ui.Element) {
	src := ui.Local(area, "onevra.chart.tip", func() tipSource { return tipSource{index: -1} })
	x, y, over := area.PointerPosition()
	moved := over && (!src.over || x != src.x || y != src.y)
	switch {
	case moved:
		src.pointer = true
	case selection.Index != src.index:
		src.pointer = false
	}
	src.x, src.y, src.over, src.index = x, y, over, selection.Index
	if selection.Index < 0 || (src.pointer && !over) || (!src.pointer && !area.Focused()) {
		return
	}
	var anchor ui.Element
	if src.pointer {
		anchor = ui.Box(c).Absolute().Left(x).Top(y).Size(0, 0)
	} else {
		anchor = mark()
	}
	anchor.PassThrough()
	t := c.Theme()
	ui.Overlay(c, func() {
		ui.Column(c).Absolute().PassThrough().Role(ui.RoleTooltip).
			AttachTo(anchor, ui.AnchorEnd, ui.AnchorStart).MarginStart(t.Space(3)).
			MinWidth(t.Space(30)).MaxWidth(t.Space(80)).Padding(t.Space(2), t.Space(2.5)).Gap(t.Space(1)).
			Radius(t.Radius).Material(glass.Glass{Tint: t.Background.Alpha(.92)}).Border(1, t.Border).
			TextColor(t.Text).FontSize(t.FontSize-1).
			Shadow(0, t.Space(1), t.Space(4), 0, ui.RGBA(0, 0, 0, .14)).Children(func() {
			if opts.Detail != nil {
				opts.Detail(c, selection)
				return
			}
			components.Text(c, selection.Label).Bold()
			for _, v := range selection.Values {
				ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					swatch(c, v.Color)
					components.Text(c, v.Label).TextColor(t.TextMuted).MinWidth(0).Shrink(1).NoWrap().Ellipsis("…")
					ui.Box(c).Grow(1)
					components.Text(c, format(opts, v.Value)).Bold().NoWrap()
				})
			}
		})
	})
}
