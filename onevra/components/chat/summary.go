package chat

import (
	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// SummaryItem describes an artifact or result; metadata and activation are
// application-owned. IDs must be nonempty and unique within the summary.
type SummaryItem struct {
	ID, Label, Detail string
	// Metadata replaces Detail with application-built badges or metrics.
	Metadata View
}

type SummaryState struct{ Expanded bool }

// SummaryOptions supplies already-prepared result information. The compact view
// shows three items; expansion shows at most 100. MoreLabel and LessLabel must
// describe the actual visible limits for the application's collection. Action
// remains available for opening the full result in an application-owned view.
type SummaryOptions struct {
	Title, Description, ActionLabel, MoreLabel, LessLabel string
	Items                                                 []SummaryItem
	// Metadata replaces Description; builders run only for visible items.
	Metadata View
}

// SummaryResult reports frame-local actions; it performs no file or network I/O.
type SummaryResult struct {
	Root       ui.Element
	Action     bool
	SelectedID string
}

// ResultSummary displays a bounded artifact list outside the execution fold.
func ResultSummary(c *ui.Context, state *SummaryState, options SummaryOptions) SummaryResult {
	var result SummaryResult
	t := c.Theme()
	result.Root = ui.Column(c).FillWidth().MinWidth(0).Shrink(0).Radius(t.Radius*1.5).Border(1, t.Border).Background(t.Text.Alpha(.035)).Children(func() {
		ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(3)).Padding(t.Space(3)).Children(func() {
			components.Icon(c, lucide.Files).Size(t.Space(5), t.Space(5)).Shrink(0).TextColor(t.TextMuted)
			ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
				components.Text(c, options.Title).Bold()
				if options.Metadata != nil {
					ui.Row(c).Gap(t.Space(2)).Children(func() { options.Metadata(c) })
				} else if options.Description != "" {
					components.Text(c, options.Description).TextColor(t.TextMuted)
				}
			})
			if options.ActionLabel != "" {
				result.Action = components.Button(c, options.ActionLabel, components.ButtonOutline).Clicked()
			}
		})
		limit := 3
		if state.Expanded {
			limit = 100
		}
		for _, item := range options.Items[:min(len(options.Items), limit)] {
			ui.Column(c).Key(item.ID).FillWidth().MinWidth(0).Children(func() {
				b := components.Button(c, "", components.ButtonGhost).FillWidth().MinWidth(0).Radius(0).Label(item.Label).Padding(t.Space(2), t.Space(3))
				b.Children(func() {
					components.Text(c, item.Label).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
					if item.Metadata != nil {
						ui.Row(c).Gap(t.Space(2)).Shrink(0).Children(func() { item.Metadata(c) })
					} else {
						components.Text(c, item.Detail).TextColor(t.TextMuted).Shrink(0)
					}
				})
				if b.Clicked() {
					result.SelectedID = item.ID
				}
			})
		}
		if len(options.Items) > 3 {
			label := options.MoreLabel
			if state.Expanded {
				label = options.LessLabel
			}
			if components.Button(c, label, components.ButtonGhost).FillWidth().Radius(0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Clicked() {
				state.Expanded = !state.Expanded
				pauseConversation(c)
			}
		}
	})
	return result
}
