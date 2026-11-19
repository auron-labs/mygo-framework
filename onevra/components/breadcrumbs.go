package components

import (
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// BreadcrumbsControl retains activation separately from Element styling.
type BreadcrumbsControl struct {
	ui.Element
	changed bool
}

// Changed reports activation of an ancestor, even if chosen already held its index.
func (b *BreadcrumbsControl) Changed() bool { return b.changed }

// Breadcrumbs displays a path with interactive ancestors and a non-interactive
// final item. Activation writes the ancestor index to chosen; applications own
// navigation and the path itself. A nil chosen binding is invalid.
// Keep the control to query Changed after applying Element layout methods.
//
// The path fills its parent's width. When it does not fit, the ancestors after
// the first collapse, nearest last, into an ellipsis button whose menu lists
// them in order; the first and final items stay and truncate if still too wide.
func Breadcrumbs(c *ui.Context, items []string, chosen *int) *BreadcrumbsControl {
	if chosen == nil {
		panic("components.Breadcrumbs: nil chosen")
	}
	t := c.Theme()
	b := &BreadcrumbsControl{Element: ui.Row(c).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(t.Space(.5)).Role(ui.RoleGroup)}
	// The width is the last frame's layout, which the first frame lacks: a
	// path that could collapse hides rather than show uncollapsed, and builds
	// again once laid out.
	hidden := 0
	if len(items) > 2 {
		if r := b.LayoutBounds(); r.W > 0 {
			hidden = breadcrumbsHidden(c, items, r.W)
		} else {
			b.Invisible()
			c.AnimationFrame()
		}
	}
	activate := func(i int) { *chosen, b.changed = i, true }
	separator := func() {
		Icon(c, lucide.ChevronRight).Size(t.Space(3), t.Space(3)).Shrink(0).TextColor(t.TextMuted).Role(ui.RoleNone)
	}
	b.Element.Children(func() {
		for i, item := range items {
			if hidden > 0 && i > 0 && i <= hidden {
				if i == 1 {
					separator()
					breadcrumbsMore(c, items[1:hidden+1], func(j int) { activate(1 + j) })
				}
				continue
			}
			if i > 0 {
				separator()
			}
			if i == len(items)-1 {
				// Match the links' padding and border so the path keeps its
				// height and text position as items become current.
				Text(c, item).SingleLine().MinWidth(0).FontWeight(500).Padding(t.Space(.5)+1, t.Space(1.5)+1)
				continue
			}
			ui.Box(c).Key(i).MinWidth(0).Children(func() {
				link := Button(c, item, ButtonGhost).Role(ui.RoleLink).MinWidth(0).
					Padding(t.Space(.5), t.Space(1.5)).TextColor(t.TextMuted)
				if link.Clicked() {
					activate(i)
				}
			})
		}
	})
	return b
}

// breadcrumbsMore is the ellipsis menu button standing in for the collapsed
// ancestors, reporting the chosen one's index among them.
func breadcrumbsMore(c *ui.Context, items []string, choose func(int)) {
	t := c.Theme()
	ui.Box(c).Key(breadcrumbsMoreKey{}).Shrink(0).Children(func() {
		more := Button(c, "", ButtonGhost).Label(l10n.In(c.Locale()).Text("breadcrumbsMore")).
			Padding(t.Space(.5), t.Space(1.5)).TextColor(t.TextMuted)
		more.Children(func() { Icon(c, lucide.Ellipsis).Size(t.Space(4), t.Space(4)) })
		more.Menu(func(m *ui.Menu) {
			for j, item := range items {
				if m.Item(item).Chosen() {
					choose(j)
				}
			}
		})
	})
}

// breadcrumbsMoreKey keys the ellipsis button's slot.
type breadcrumbsMoreKey struct{}

// breadcrumbsHidden returns how many ancestors after the first collapse so the
// path fits width: none when it all fits, otherwise the fewest that do, up to
// every ancestor between the first and final items.
func breadcrumbsHidden(c *ui.Context, items []string, width float32) int {
	t := c.Theme()
	// Each item's text plus its padding and border, and each separator with
	// its gaps.
	inset := 2 * (t.Space(1.5) + 1)
	separator := t.Space(3) + 2*t.Space(.5)
	widths := make([]float32, len(items))
	total := float32(0)
	for i, item := range items {
		span := ui.Span{Text: item}
		if i == len(items)-1 {
			span.Weight = 500
		}
		w, _ := c.MeasureText(0, span)
		widths[i] = w + inset
		total += widths[i]
	}
	total += separator * float32(len(items)-1)
	if total <= width {
		return 0
	}
	// Collapsing replaces the hidden items and their separators with the
	// ellipsis and one separator.
	total += t.Space(4) + inset + separator
	hidden := 0
	for hidden < len(items)-2 && total > width {
		hidden++
		total -= widths[hidden] + separator
	}
	return hidden
}
