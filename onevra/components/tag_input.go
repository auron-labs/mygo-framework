package components

import (
	"slices"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// TagInputControl exposes layout on Element and editing on the draft field.
// Keep this handle to query Changed after applying Element layout methods.
type TagInputControl struct {
	ui.Element
	input   ui.Element
	empty   bool
	changed bool
}

// Changed reports a tag added or removed by the user in this frame.
func (n *TagInputControl) Changed() bool { return n.changed }

// Label names the draft field. Remove buttons are named after their tags.
func (n *TagInputControl) Label(label string) *TagInputControl {
	n.input.Label(label)
	return n
}

// Placeholder shows text in the field while it holds no tags.
func (n *TagInputControl) Placeholder(text string) *TagInputControl {
	if n.empty {
		n.input.Placeholder(text)
	}
	return n
}

// AutoFocus focuses the draft field.
func (n *TagInputControl) AutoFocus() *TagInputControl {
	n.input.AutoFocus()
	return n
}

// Motion follows the reference design's tokens: tags morph, the field height
// glides without overshooting and the pick ring settles quickly.
var (
	tagMorph    = ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond, Bounce: .16}}
	tagSmooth   = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}
	tagSnappy   = ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: .12}}
	tagEnter    = ui.CubicBezier(.16, 1, .3, 1)
	tagStandard = ui.CubicBezier(.22, 1, .36, 1)
	tagInOut    = ui.CubicBezier(.65, 0, .35, 1)
	// New tags scale in where the text was typed; the rest glide aside.
	tagTransition = ui.ElementTransition{
		Animation: &tagMorph, Position: true,
		Enter: &ui.Motion{Scale: .9}, Exit: &ui.Motion{Scale: .9},
	}
	tagBlurIn = []ui.Keyframe{{Value: 4}, {At: 240 * time.Millisecond, Value: 0, Ease: tagEnter}}
	tagPulse  = []ui.Keyframe{
		{Value: 1},
		{At: 180 * time.Millisecond, Value: 1.06, Ease: tagInOut},
		{At: 360 * time.Millisecond, Value: 1, Ease: tagInOut},
	}
)

// Distinct key types keep a tag named like an internal part from sharing its identity.
type (
	tagKey      struct{ tag string }
	tagRingKey  struct{}
	tagDraftKey struct{}
)

type tagInputState struct {
	draft string
	// picked is the tag the next Backspace or Delete removes; "" for none.
	picked, announced string
	// Key handling runs between frames: it reads the tags as last built,
	// moves the pick at once and leaves removals and commits to the build.
	tags     []string
	removals []string
	commit   bool
	atStart  bool
	rtl      bool
	focused  bool
	// built holds the tags shown in the previous frame, so only new ones
	// blur in after the first; blurring and pulses hold their running animations.
	ready     bool
	built     map[string]bool
	blurring  map[string]bool
	pulses    map[string]uint64
	ring      ui.Rect
	ringShown bool
}

// key handles a key the draft field sees before editing it, reporting
// whether it took the key.
func (s *tagInputState) key(k ui.TextKey) bool {
	if k.Mods != 0 {
		return false
	}
	back, forward := ui.KeyLeft, ui.KeyRight
	if s.rtl {
		back, forward = forward, back
	}
	index := len(s.tags)
	if s.picked != "" {
		index = slices.Index(s.tags, s.picked)
	}
	start := k.Text() == "" || s.atStart
	switch {
	case k.Key == ui.KeyEnter:
		if strings.TrimSpace(k.Text()) == "" {
			return false
		}
		s.commit = true
	case (k.Key == ui.KeyBackspace || k.Key == ui.KeyDelete) && s.picked != "":
		s.removals = append(s.removals, s.picked)
		s.tags = slices.Delete(slices.Clone(s.tags), index, index+1)
		s.picked = ""
	case k.Key == ui.KeyBackspace && start && len(s.tags) > 0:
		s.picked = s.tags[len(s.tags)-1]
	case k.Key == back && (start || s.picked != "") && index > 0:
		s.picked = s.tags[index-1]
	case k.Key == forward && s.picked != "":
		s.picked = ""
		if index+1 < len(s.tags) {
			s.picked = s.tags[index+1]
		}
	case k.Key == ui.KeyEscape && s.picked != "":
		s.picked = ""
	default:
		return false
	}
	return true
}

// TagInput edits application-owned *tags, distinct plain strings shown as
// removable pills before a draft field. Enter or a comma adds the trimmed draft, as
// does leaving the field; pasted comma-separated text adds each part. A
// duplicate, compared without case, pulses the existing tag instead; Enter
// keeps such a draft for correction. Backspace at the start of the draft
// picks the last tag and removes it when pressed again; Delete removes a
// picked tag, the arrow keys move the pick and Escape clears it. Clicking a
// tag picks it. Announcements report additions, removals and picks.
//
// Tags wrap onto new rows while the field's height follows on a spring. New
// tags blur and scale in, removed tags leave and the rest glide into place;
// Reduced Motion skips the movement. Applications validate or normalize tags
// after Changed. Each tag animates individually, which suits dozens of tags,
// not hundreds. Key a parent for stable identity; construction resolves
// interaction state.
func TagInput(c *ui.Context, tags *[]string) *TagInputControl {
	if tags == nil {
		panic("components.TagInput: tags required")
	}
	t := c.Theme()
	words := localized(c).messages
	reduce := c.Preferences().ReduceMotion
	n := &TagInputControl{Element: ui.Box(c).MinWidth(0).Radius(t.Radius).Clip()}
	state := *ui.Local(n.Element, "tag-input", func() *tagInputState {
		return &tagInputState{built: map[string]bool{}, blurring: map[string]bool{}, pulses: map[string]uint64{}}
	})
	state.rtl = c.Direction() == ui.RightToLeft

	set := func(next []string) {
		*tags = next
		n.changed = true
	}
	remove := func(tag string) {
		if i := slices.Index(*tags, tag); i >= 0 {
			set(slices.Concat((*tags)[:i], (*tags)[i+1:]))
			c.Announce(words.Label("tagRemoved", tag))
		}
	}
	// add reports whether text was taken: added, or empty.
	add := func(text string) bool {
		tag := strings.TrimSpace(text)
		if tag == "" {
			return true
		}
		if i := slices.IndexFunc(*tags, func(s string) bool { return strings.EqualFold(s, tag) }); i >= 0 {
			state.pulses[(*tags)[i]]++
			c.Announce(words.Label("tagExists", (*tags)[i]))
			return false
		}
		set(append(slices.Clip(*tags), tag))
		state.picked = ""
		c.Announce(words.Label("tagAdded", tag))
		return true
	}

	for _, tag := range state.removals {
		remove(tag)
	}
	state.removals = nil
	if state.commit && add(state.draft) {
		state.draft = ""
	}
	state.commit = false
	if !slices.Contains(*tags, state.picked) {
		state.picked = ""
	}
	if state.picked != state.announced && state.picked != "" {
		c.Announce(words.Label("tagSelected", state.picked))
	}
	state.announced = state.picked

	border := t.Border
	if !n.IsDisabled() {
		switch {
		case n.FocusWithin():
			border = t.Accent
		case n.Hovered():
			border = t.Border.Mix(t.Text, 0.25)
		}
	}
	n.Background(t.Surface.Alpha(0.5)).Border(1, n.AnimateColor("border", border, ui.Animation{Duration: 160 * time.Millisecond, Ease: tagStandard}))
	n.Transition(ui.ElementTransition{Animation: &tagSmooth, Size: true})

	// Pills fill a TextBox's line, so one row matches its height. Under a
	// coarse pointer the line takes TouchMin's square, for the finger.
	_, line := c.MeasureText(0, ui.Span{Text: "Ag"})
	height, inset := max(line+t.Space(1.5), TouchMin(c)), t.Space(.75)
	n.Children(func() {
		row := ui.Row(c).FillWidth().Shrink(0).Wrap().AlignItems(ui.Center).AlignContent(ui.Start).
			Gap(t.Space(1.5)).Padding(inset).Cursor(ui.CursorText)
		var removed string
		var picked ui.Element
		refocus, toggled := false, ""
		row.Children(func() {
			shown := make(map[string]bool, len(*tags))
			for _, tag := range *tags {
				shown[tag] = true
				chip := ui.Row(c.Key(tagKey{tag})).Height(height).MinWidth(0).MaxWidthPercent(100).Shrink(0).
					AlignItems(ui.Center).Gap(t.Space(.5)).PaddingStart(t.Space(2.25)).PaddingEnd(t.Space(.75)).
					Radius(height/2).Background(t.Background.Mix(t.Text, .05)).Border(1, t.Border).
					Cursor(ui.CursorDefault).KeepFocus().Transition(tagTransition)
				if tag == state.picked {
					picked = chip
				}
				if state.ready && !state.built[tag] {
					state.blurring[tag] = true
				}
				if state.blurring[tag] {
					// Keyframes run on their own and end at zero under Reduced Motion.
					blur := chip.Keyframes("blur", tagBlurIn)
					if blur > .01 {
						chip.Blur(blur)
					} else {
						delete(state.blurring, tag)
					}
				}
				if run := state.pulses[tag]; run > 0 {
					scale := chip.Keyframes("pulse", tagPulse, ui.KeyframeOptions{Run: run})
					chip.TransformOrigin(.5, .5).Scale(scale, scale)
				}
				chip.Children(func() {
					Text(c, tag).SingleLine().Ellipsis("…").MinWidth(0).Unselectable()
					size := max(t.Space(4), TouchMin(c))
					b := ui.ButtonBase(c).Label(words.Label("removeTag", tag)).Size(size, size).Shrink(0).
						Radius(size / 2).Center().KeepFocus().Cursor(ui.CursorPointer)
					color := t.TextMuted
					if tag == state.picked {
						color = t.Text
					}
					if !b.IsDisabled() && (b.Hovered() || b.FocusVisible()) {
						b.Background(t.Surface)
						color = t.Text
					}
					pose := ui.Pose{}
					if b.Pressed() {
						pose.Scale = .96
					}
					b.AnimatePose(pose, tagSnappy)
					b.Children(func() { Icon(c, lucide.X).Size(t.Space(3), t.Space(3)).TextColor(color) })
					if b.Clicked() {
						removed, refocus = tag, true
					}
				})
				if chip.Clicked() && removed != tag {
					toggled, refocus = tag, true
				}
			}
			state.built, state.ready = shown, true
			for tag := range state.blurring {
				if !shown[tag] {
					delete(state.blurring, tag)
				}
			}
			for tag := range state.pulses {
				if !shown[tag] {
					delete(state.pulses, tag)
				}
			}

			// The ring rides above the pills and glides between picks; it
			// leaves on the same curve as a removed tag.
			ring := ui.Box(c.Key(tagRingKey{})).Absolute().PassThrough().TransformOrigin(.5, .5).
				Radius(height/2).Border(1, t.Accent).Background(t.Accent.Alpha(.12))
			show := picked.Valid() && picked.LayoutBounds().W > 0
			if show {
				b, r := picked.LayoutBounds(), row.LayoutBounds()
				state.ring = ui.Rect{X: b.X - r.X, Y: b.Y - r.Y, W: b.W, H: b.H}
			}
			box := state.ring
			var opacity, scale float32
			if show {
				opacity = ring.AnimateWith("opacity", 1, 160*time.Millisecond, tagEnter)
			} else {
				opacity = ring.AnimateWith("opacity", 0, 120*time.Millisecond, tagStandard)
			}
			if show && !state.ringShown && opacity < .02 {
				// A ring arriving from nothing scales in where the pick is.
				box.X, box.Y = ring.Jump("x", box.X), ring.Jump("y", box.Y)
				box.W, box.H = ring.Jump("w", box.W), ring.Jump("h", box.H)
				scale = 1
				if !reduce {
					scale = ring.Jump("scale", .9)
				}
				c.AnimationFrame()
			} else {
				box.X, box.Y = ring.AnimateTo("x", box.X, tagMorph), ring.AnimateTo("y", box.Y, tagMorph)
				box.W, box.H = ring.AnimateTo("w", box.W, tagMorph), ring.AnimateTo("h", box.H, tagMorph)
				target := float32(.9)
				if show || reduce {
					target = 1
				}
				scale = ring.AnimateTo("scale", target, tagSnappy)
			}
			state.ringShown = show
			ring.Left(box.X).Top(box.Y).Size(box.W, box.H).Opacity(opacity).Scale(scale, scale)

			n.input = ui.TextInputBase(c.Key(tagDraftKey{}), &state.draft).Grow(1).Basis(t.Space(25)).MinWidth(t.Space(20)).
				Height(height).Padding(0, t.Space(1.25)).FocusRing(false).
				Transition(ui.ElementTransition{Animation: &tagMorph, Position: true}).OnTextKey(state.key)
		})
		in := n.input
		if in.Changed() {
			state.picked = ""
		}
		if strings.Contains(state.draft, ",") {
			parts := strings.Split(state.draft, ",")
			for _, part := range parts[:len(parts)-1] {
				add(part)
			}
			state.draft = parts[len(parts)-1]
		}
		focused := in.Focused()
		if state.focused && !focused {
			if add(state.draft) {
				state.draft = ""
			}
			state.picked = ""
		}
		state.focused = focused
		if removed != "" {
			remove(removed)
			state.picked = ""
		}
		if toggled != "" {
			if state.picked == toggled {
				state.picked = ""
			} else {
				state.picked = toggled
			}
		}
		if row.Clicked() {
			refocus = true
		}
		if refocus && !focused {
			in.Focus()
		}
		a, b := in.TextSelection()
		state.atStart = a == 0 && b == 0
		state.tags = slices.Clone(*tags)
		if n.changed || toggled != "" {
			c.Invalidate()
		}
	})
	n.empty = len(*tags) == 0
	dimmed(n.Element)
	return n
}
