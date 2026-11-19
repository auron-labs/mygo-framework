package components

import (
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// AvatarGroupMember is one person in an AvatarGroup.
type AvatarGroupMember struct {
	// Name is the accessible name, hover hint and source of the initials.
	// Members keep their identity by name, and by order among equal names.
	Name   string
	Image  ui.ImageSource
	Status AvatarStatus
}

// AvatarGroupOptions customize an AvatarGroup.
type AvatarGroupOptions struct {
	// Max is how many avatars show before the rest collapse into one "+N"
	// chip: 4 when zero; a negative Max collapses everyone.
	Max int
	// Size is the avatar and chip diameter in DIPs; zero uses Avatar's
	// standard size.
	Size float32
	// Overlap is how far each avatar covers the one before it, in DIPs: a
	// sixth of the diameter when zero; a negative value leaves that much
	// space between avatars instead.
	Overlap float32
	// Reverse puts the first member in front, so each avatar tucks behind
	// the one before it instead of covering it.
	Reverse bool
	// Label names the group for assistive technology; empty uses the
	// localized "Team members".
	Label string
}

const avatarGroupDuration = 200 * time.Millisecond

// avatarGroupKey keys a member's slot by name and occurrence of that name.
type avatarGroupKey struct {
	name string
	n    int
}

// avatarGroupOverflowKey keys the overflow chip's slot.
type avatarGroupOverflowKey struct{}

// avatarGroupEntry is one slot to build, in paint order.
type avatarGroupEntry struct {
	key      any
	member   AvatarGroupMember
	overflow int
}

// avatarGroupCount keeps the last overflow count and the way it moved.
type avatarGroupCount struct{ count, direction int }

// AvatarGroup creates an overlapping stack of members in order. Members past
// Max collapse into an overflow chip, so long lists build only what shows.
// Pointing at the stack loosens it and lifts the member under the pointer,
// which names itself in a tooltip; members joining or leaving open or close
// their slots and the overflow count rolls the way it moved. Reduced Motion
// keeps the stack still. Overlap and Reverse adjust how the avatars cover one
// another. Applications own members, photos and localized labels. Key a
// parent for identity.
func AvatarGroup(c *ui.Context, members []AvatarGroupMember, options AvatarGroupOptions) ui.Element {
	t := c.Theme()
	d := avatarDiameter(c, options.Size)
	limit := options.Max
	if limit == 0 {
		limit = 4
	}
	limit = min(max(0, limit), len(members))
	overflow := len(members) - limit
	label := options.Label
	if label == "" {
		label = l10n.In(c.Locale()).Text("avatarGroup")
	}
	overlap := avatarGroupOverlap(options.Overlap, d)
	reduce := c.Preferences().ReduceMotion
	slotMotion := ui.ElementTransition{
		Animation: &ui.Smooth,
		Enter:     &ui.Motion{Collapse: true, Scale: .9},
		Exit:      &ui.Motion{Collapse: true, Scale: .9},
	}
	group := ui.Row(c).Height(d).Shrink(0).AlignSelf(ui.Start)
	if options.Reverse {
		group.Reverse()
	}
	switch {
	case overlap > 0:
		group.PaddingStart(overlap)
	case overlap < 0:
		group.Gap(-overlap)
	}
	group.Role(ui.RoleGroup).Label(label)
	fanned := group.Hovered() && !reduce
	group.Children(func() {
		var slots []ui.Element
		hovered := -1
		slot := func(key any) ui.Element {
			s := ui.Row(c).Key(key).Height(d).Shrink(0)
			if overlap > 0 {
				s.MarginStart(-overlap)
			}
			s.Transition(slotMotion)
			if s.Hovered() {
				hovered = len(slots)
			}
			slots = append(slots, s)
			return s
		}
		// Hovering turns the separator in front of the member into the
		// accent rather than adding another outline over it.
		ring := func(e ui.Element, hover bool) {
			if hover {
				e.Border(2, t.Accent)
			}
		}
		seen := map[string]int{}
		entries := make([]avatarGroupEntry, 0, limit+1)
		for _, m := range members[:limit] {
			entries = append(entries, avatarGroupEntry{key: avatarGroupKey{m.Name, seen[m.Name]}, member: m})
			seen[m.Name]++
		}
		if overflow > 0 {
			entries = append(entries, avatarGroupEntry{key: avatarGroupOverflowKey{}, overflow: overflow})
		}
		// The stack paints its entries in order, so the last built covers
		// the ones before it: reversing puts the first member in front.
		if options.Reverse {
			slices.Reverse(entries)
		}
		for _, entry := range entries {
			s := slot(entry.key)
			hover := hovered == len(slots)-1
			if entry.overflow > 0 {
				s.Children(func() {
					chip := ui.Box(c).Size(d, d).Shrink(0).Center().Radius(d/2).Clip().
						Background(t.SurfaceHover).TextColor(t.TextMuted).Border(2, t.Surface).
						Role(ui.RoleImage).Label(l10n.In(c.Locale()).Count("avatarGroupMore", entry.overflow))
					ring(chip, hover)
					last := ui.Local(chip, "count", func() avatarGroupCount {
						return avatarGroupCount{entry.overflow, 1}
					})
					if last.count != entry.overflow {
						last.direction = 1
						if entry.overflow < last.count {
							last.direction = -1
						}
						last.count = entry.overflow
					}
					// More people rise in from below, fewer drop in from above.
					shift := d * .2 * float32(last.direction)
					chip.Children(func() {
						ui.Text(c, "+"+strconv.Itoa(entry.overflow)).Key(entry.overflow).SingleLine().Unselectable().
							FontSize(avatarFontSize(d)).FontWeight(500).
							Transition(ui.ElementTransition{
								Duration: avatarGroupDuration,
								Enter:    &ui.Motion{Y: shift},
								Exit:     &ui.Motion{Y: -shift},
							})
					})
				})
				continue
			}
			s.Children(func() {
				a := avatar(c, entry.member.Name, AvatarOptions{
					Image: entry.member.Image, Status: entry.member.Status, Size: d,
				}, 2)
				ring(a, hover)
				// The tooltip repeats the accessible name, so it adds no description.
				Tooltip(c, a, entry.member.Name)
				a.Description("")
			})
		}
		if reduce {
			return
		}
		// Hovering loosens the overlap from the center; the hovered member
		// rises and its neighbors ease aside. Reverse builds the slots in the
		// other order, so its offsets turn the other way.
		spread := overlap
		if spread < 0 {
			spread = -spread
		}
		direction := float32(1)
		if options.Reverse {
			direction = -1
		}
		for i, s := range slots {
			var x, y float32
			if fanned {
				x = (float32(i) - float32(len(slots)-1)/2) * spread * 2 / 3 * direction
			}
			switch {
			case hovered < 0:
			case i == hovered:
				y = -2
			case i > hovered:
				x += spread / 2 * direction
			default:
				x -= spread / 2 * direction
			}
			motion := ui.Animation{Duration: avatarGroupDuration, Ease: ui.EaseInOut}
			s.Translate(s.AnimateTo("fanX", x, motion), s.AnimateTo("fanY", y, motion))
		}
	})
	return group
}

// avatarGroupOverlap resolves AvatarGroupOptions.Overlap for avatars of
// diameter d: zero or a non-finite value uses a sixth of the diameter, and
// no avatar covers more than the whole one before it.
func avatarGroupOverlap(v, d float32) float32 {
	if v == 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return d / 6
	}
	return min(v, d)
}
