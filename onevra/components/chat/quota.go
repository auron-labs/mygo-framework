package chat

import (
	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// QuotaRow is one account limit, such as Label "5-hour limit", Detail
// "Resets in 2h" and Value "63% left". Fraction is the remaining share, 0..1;
// a negative Fraction omits the bar, as for a balance.
type QuotaRow struct {
	Label, Detail, Value string
	Fraction             float64
}

// QuotaOptions describes one account's limits. ID is stable within a
// QuotaSummary. Leading precedes Title, such as a provider mark. The header's
// trailing edge shows Countdown, such as the time to the next automatic read
// ("04:59") or "Refreshing…", then a refresh action when RefreshLabel is set
// and an open action, such as the provider's usage page, when OpenLabel is
// set; Refreshing disables refreshing. Trailing adds further controls. Note
// explains missing or stale rows, and Loading shows placeholders while no
// rows have been read.
type QuotaOptions struct {
	ID, Title, Note         string
	Countdown               string
	RefreshLabel, OpenLabel string
	Refreshing              bool
	Leading                 func(c *ui.Context, size float32)
	Trailing                View
	Rows                    []QuotaRow
	Loading                 bool
}

// QuotaResult reports frame-local requests; the application reads usage and
// opens pages itself.
type QuotaResult struct {
	Root          ui.Element
	Refresh, Open bool
}

// Quota lists an account's limits with accent bars of what remains, warning
// from 20% left and alerting from 5%. The application reads and refreshes usage.
func Quota(c *ui.Context, options QuotaOptions) QuotaResult {
	var result QuotaResult
	t := c.Theme()
	result.Root = ui.Column(c).FillWidth().MinWidth(0).Gap(7).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).Height(20).AlignItems(ui.Center).Gap(6).Children(func() {
			if options.Leading != nil {
				options.Leading(c, 13)
			}
			components.Text(c, options.Title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5).TextColor(tertiary(t))
			if options.Countdown != "" {
				components.Text(c, options.Countdown).Shrink(0).SingleLine().FontSize(11.5).TextColor(tertiary(t))
			}
			if options.RefreshLabel != "" {
				result.Refresh = quotaAction(c, options.RefreshLabel, lucide.RefreshCw).Disabled(options.Refreshing).Clicked() && !options.Refreshing
			}
			if options.OpenLabel != "" {
				result.Open = quotaAction(c, options.OpenLabel, lucide.ExternalLink).Clicked()
			}
			if options.Trailing != nil {
				options.Trailing(c)
			}
		})
		for i, row := range options.Rows {
			ui.Column(c).Key(i).FillWidth().MinWidth(0).Gap(7).Children(func() {
				ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(6).Children(func() {
						components.Text(c, row.Label).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5)
						if row.Detail != "" {
							components.Text(c, row.Detail).Shrink(0).FontSize(11.5).TextColor(tertiary(t))
						}
					})
					components.Text(c, row.Value).Shrink(0).FontSize(12.5).TextColor(secondary(t))
				})
				if row.Fraction >= 0 {
					bar(c, row.Fraction, quotaColor(t, row.Fraction)).FillWidth().Height(6)
				}
			})
		}
		if options.Loading && len(options.Rows) == 0 {
			for range 2 {
				ui.Box(c).FillWidth().Height(6).Radius(3).Background(overlay(t))
			}
		}
		if options.Note != "" {
			components.Text(c, options.Note).FillWidth().MinWidth(0).Wrap().FontSize(12).TextColor(tertiary(t))
		}
	})
	return result
}

// quotaAction is a 20-point icon button with a tooltip in a Quota header.
func quotaAction(c *ui.Context, label string, icon func() *ui.SVG) ui.Element {
	b := components.Button(c, "", components.ButtonGhost).Label(label).Size(20, 20).Padding(0).Shrink(0).Center()
	b.Children(func() { components.Icon(c, icon).Size(13, 13).Shrink(0) })
	components.Tooltip(c, b, label)
	return b
}

func quotaColor(t *ui.Theme, remaining float64) ui.Color {
	switch {
	case remaining <= .05:
		return t.Danger
	case remaining <= .2:
		return t.Warning
	}
	return t.Accent
}

// QuotaSummaryOptions describes the limit to glance at, usually the one that
// applies to the chosen model, and the accounts the panel lists. Label is the
// accessible name and Tooltip the hover text.
type QuotaSummaryOptions struct {
	Label, Tooltip string
	Row            QuotaRow
	Quotas         []QuotaOptions
}

// QuotaSummaryResult reports the ID of a Quota whose refresh or open action
// was chosen this frame.
type QuotaSummaryResult struct {
	Root          ui.Element
	Refresh, Open string
}

// QuotaSummary is a compact footer control that shows Row's Detail, such as
// the time to reset, and a small accent bar of what remains, or Value for a
// balance.
// Clicking opens a panel with every Quota. The application chooses the row.
func QuotaSummary(c *ui.Context, state *MeterState, options QuotaSummaryOptions) QuotaSummaryResult {
	var result QuotaSummaryResult
	t := c.Theme()
	row := options.Row
	b := ui.ButtonBase(c).Label(options.Label).Height(20).Padding(0, 5).Radius(5).Shrink(0).AlignItems(ui.Center).Gap(6).
		Background(ui.Color{})
	if state.Open {
		b.Background(overlayStrong(t))
	} else if b.Hovered() || b.Focused() {
		b.Background(overlay(t))
	}
	b.Children(func() {
		if row.Fraction < 0 {
			components.Text(c, row.Value).Shrink(0).SingleLine().FontSize(11.5).TextColor(tertiary(t))
			return
		}
		if row.Detail != "" {
			components.Text(c, row.Detail).Shrink(0).SingleLine().FontSize(11.5).TextColor(tertiary(t))
		}
		bar(c, row.Fraction, t.Accent).Size(48, 4)
	})
	if options.Tooltip != "" && !state.Open {
		components.Tooltip(c, b, options.Tooltip)
	}
	if b.Clicked() {
		state.Open = !state.Open
	}
	components.Popover(c, b, &state.Open, func(panel ui.Element) {
		panel.Label(options.Label).Width(320).Padding(14).Gap(14).Children(func() {
			for i, q := range options.Quotas {
				ui.Column(c).Key(i).FillWidth().MinWidth(0).Children(func() {
					r := Quota(c, q)
					if r.Refresh {
						result.Refresh = q.ID
					}
					if r.Open {
						result.Open = q.ID
					}
				})
			}
		})
	})
	result.Root = b
	return result
}
