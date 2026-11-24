// Package chat composes native conversation views from Onevra controls and
// MyGo primitives. Applications own messages, stable IDs, localized labels,
// parsed documents and asynchronous work. Builders run on the UI thread and
// must not start work. Retain state per conversation and turn, not per frame.
// Sidebar assembles these views into an assistant panel and names its own
// controls from Onevra's catalog.
package chat

import (
	"math"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// Status describes a turn or activity's application-reported lifecycle. A
// tool finishing does not finish its turn. Only Completed permits folding.
type Status uint8

const (
	Running Status = iota
	Waiting
	Completed
	Failed
	Cancelled
)

// View builds arbitrary content, including retained Markdown documents,
// attachments, confirmation controls or bounded tool-output previews.
type View func(*ui.Context)

// Activity is one execution step inside an activity group, such as reading
// a file, running a command or thinking. ID is stable within its turn; update
// a tool's existing activity by its call ID, even if results arrive out of
// order. Action is a short verb ("Read") and Detail its target.
type Activity struct {
	ID, Action, Detail string
	Icon               func() *ui.SVG
	Status             Status
	// Content is the expanded detail. Nil keeps the activity a single row.
	Content View
	// Mono presents Content as monospace output rather than prose.
	Mono bool
	// State retains expansion and must be non-nil when Content is set.
	State *ActivityState
	// MaxHeight bounds the expanded detail in DIPs; zero uses 400.
	MaxHeight float32
}

// ActivityState belongs to one stable activity ID.
type ActivityState struct {
	Expanded bool
	Scroll   ui.ScrollState
}

// GroupState retains an activity group's explicit expansion. Its zero value
// opens the group while it is live and closes it afterwards.
type GroupState struct{ choice uint8 }

func (g *GroupState) expanded(live bool) bool {
	if g == nil || g.choice == 0 {
		return live
	}
	return g.choice == 1
}

// SetExpanded records an explicit choice that survives the group settling.
func (g *GroupState) SetExpanded(open bool) { g.set(open) }

func (g *GroupState) set(open bool) {
	if g != nil {
		g.choice = 2
		if open {
			g.choice = 1
		}
	}
}

// Step is either intermediate prose (Message) or a group of activities with
// an application-summarized Title such as "Ran 2 commands".
type Step struct {
	ID         string
	Message    View
	Title      string
	Activities []Activity
	Group      *GroupState
}

// TurnState retains only presentation state. Do not share it between turns or
// copy it after first use. Its zero value follows the automatic fold policy.
type TurnState struct {
	choice uint8
	body   ui.Handle
	folded bool
}

// SetExpanded records an explicit preference that survives stream updates.
func (s *TurnState) SetExpanded(open bool) {
	s.choice = 2
	if open {
		s.choice = 1
	}
}

// ResetExpansion restores automatic folding, for example for a new attempt.
func (s *TurnState) ResetExpansion() { s.choice, s.folded = 0, false }

// Expanded resolves presentation without modifying application data. Completed
// turns fold only when a final response exists. HoldOpen temporarily protects
// reading/selection; explicit user choices always take precedence.
func (s *TurnState) Expanded(status Status, hasFinal, holdOpen bool) bool {
	if s.choice != 0 {
		return s.choice == 1
	}
	if status != Completed || !hasFinal {
		s.folded = false
		return true
	}
	if !holdOpen {
		s.folded = true
	}
	return !s.folded
}

// TurnOptions describes one request and its response. Supply a stable,
// nonempty ID. Summary labels the completed turn's fold row ("Worked for 2m")
// and Working the live indicator ("Working for 12s"). Final is explicit: the
// last step is never inferred to be the answer.
type TurnOptions struct {
	ID               string
	Status           Status
	Summary, Working string
	Prompt, Final    View
	Steps            []Step
	// Footer holds results or errors and Actions response actions such as
	// copy. Neither folds with the steps.
	Footer, Actions View
	// HoldOpen delays automatic folding while the user reads older content.
	// ConversationState.AtLatest is useful for this policy. Focus inside the
	// steps also protects them automatically, including selectable text.
	HoldOpen bool
}

// Turn displays the prompt, its steps and the final response. A completed
// turn folds its steps behind a centered summary row, building nothing hidden.
func Turn(c *ui.Context, state *TurnState, options TurnOptions) ui.Element {
	t := c.Theme()
	return ui.Column(c).Key(options.ID).FillWidth().MinWidth(0).Shrink(0).Gap(t.Space(4)).Children(func() {
		if options.Prompt != nil {
			Message(c, true, func(c *ui.Context) { Bubble(c, options.Prompt) })
		}
		scope := *conversationSlot(c)
		open := state.Expanded(options.Status, options.Final != nil, options.HoldOpen || scope.reading || state.body.FocusWithin(c))
		foldable := options.Status == Completed && options.Final != nil && len(options.Steps) > 0
		if foldable {
			if foldRow(c, options.Summary, open) {
				state.SetExpanded(!open)
				open = !open
				pauseConversation(c)
			}
		}
		if open && len(options.Steps) > 0 {
			ui.Column(c).Key("steps").Bind(&state.body).FillWidth().MinWidth(0).Gap(t.Space(4)).Children(func() {
				for i, step := range options.Steps {
					live := options.Status == Running && i == len(options.Steps)-1
					ui.Column(c).Key(step.ID).FillWidth().MinWidth(0).Children(func() {
						if step.Message != nil {
							step.Message(c)
						}
						if len(step.Activities) > 0 {
							activityGroup(c, step, live)
						}
					})
				}
			})
		}
		if options.Status == Running {
			workingRow(c, options.Working)
		}
		if options.Final != nil {
			ui.Column(c).Key("final").FillWidth().MinWidth(0).Children(func() { Message(c, false, options.Final) })
		}
		if options.Footer != nil {
			ui.Column(c).Key("footer").FillWidth().MinWidth(0).Children(func() { options.Footer(c) })
		}
		if options.Actions != nil {
			ui.Row(c).Key("actions").FillWidth().Wrap().Gap(t.Space(1)).Children(func() { options.Actions(c) })
		}
	})
}

// foldRow is the divider-centered summary of a completed turn.
func foldRow(c *ui.Context, label string, open bool) bool {
	t := c.Theme()
	var clicked bool
	ui.Row(c).Key("fold").FillWidth().Height(24).AlignItems(ui.Center).Gap(10).Children(func() {
		divider(c)
		b := ui.ButtonBase(c).Label(label).Expanded(open).Height(24).Padding(0, 2).AlignItems(ui.Center).Gap(5).
			Background(ui.Color{}).TextColor(tertiary(t)).Radius(t.Radius)
		b.Children(func() {
			components.Text(c, label).FontSize(13.5).FontWeight(500).TextColor(tertiary(t))
			icon := lucide.ChevronRight
			if open {
				icon = lucide.ChevronDown
			}
			components.Icon(c, icon).Size(11.5, 11.5).TextColor(tertiary(t))
		})
		clicked = b.Clicked()
		divider(c)
	})
	return clicked
}

// workingRow shows pulsing dots and the live elapsed label.
func workingRow(c *ui.Context, label string) {
	t := c.Theme()
	ui.Row(c).Key("working").Height(22).AlignItems(ui.Center).Gap(8).Children(func() {
		waveDots(c, tertiary(t))
		if label != "" {
			components.Text(c, label).FontSize(13.5).FontWeight(500).TextColor(tertiary(t))
		}
	})
}

func waveDots(c *ui.Context, color ui.Color) {
	reduced := c.Preferences().ReduceMotion
	ui.Box(c).Size(18, 6).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		phase := 0.0
		if !reduced {
			p.After(50 * time.Millisecond)
			phase = float64(p.Now().UnixMilli()%1200) / 1200
		}
		for i := range 3 {
			alpha := float32(.35)
			if !reduced {
				wave := math.Sin((phase - float64(i)*.15) * 2 * math.Pi)
				alpha = float32(.35 + .65*math.Max(0, wave))
			}
			x := r.X + float32(i)*6
			p.Fill(ui.Rect{X: x, Y: r.Y + (r.H-4)/2, W: 4, H: 4}, color.Alpha(alpha), 2)
		}
	})
}

// activityGroup is a disclosure header with a guide-lined list of rows.
func activityGroup(c *ui.Context, step Step, live bool) {
	t := c.Theme()
	open := step.Group.expanded(live)
	ui.Column(c).Key("activities").FillWidth().MinWidth(0).Gap(4).Children(func() {
		b := ui.ButtonBase(c).Label(step.Title).Expanded(open).FillWidth().MinWidth(0).Height(max(float32(26), components.TouchMin(c))).Justify(ui.Start).
			AlignItems(ui.Center).Gap(6).Background(ui.Color{}).TextColor(secondary(t))
		hover := b.Hovered() || b.Focused()
		b.Children(func() {
			color := secondary(t)
			if hover {
				color = t.Text
			}
			components.Text(c, step.Title).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5).FontWeight(500).TextColor(color)
			icon := lucide.ChevronRight
			if open {
				icon = lucide.ChevronDown
			}
			components.Icon(c, icon).Size(10, 10).Shrink(0).TextColor(tertiary(t))
		})
		if b.Clicked() {
			step.Group.set(!open)
			open = !open
			pauseConversation(c)
		}
		if !open {
			return
		}
		ui.Column(c).FillWidth().MinWidth(0).Margin(0, 0, 0, 6).Padding(0, 0, 2, 12).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Gap(8).Children(func() {
			for _, a := range step.Activities {
				ActivityRow(c, a)
			}
		})
	})
}

// ActivityRow renders one activity as a compact card that can expand to its
// bounded detail. Content costs and truncation are application-owned.
func ActivityRow(c *ui.Context, a Activity) ui.Element {
	t := c.Theme()
	surface := t.Background.Mix(t.Text, .035)
	detail := a.Content != nil && a.State != nil
	open := detail && a.State.Expanded
	return ui.Column(c).Key(a.ID).FillWidth().MinWidth(0).Radius(9).Border(1, strongBorder(t)).Background(surface).Clip().Children(func() {
		header := func(e ui.Element) {
			e.FillWidth().MinWidth(0).Height(28).Padding(0, 8).AlignItems(ui.Center).Gap(8)
			e.Children(func() {
				icon := a.Icon
				if icon == nil {
					icon = lucide.Wrench
				}
				components.Icon(c, icon).Size(12, 12).Shrink(0).TextColor(tertiary(t))
				components.Text(c, a.Action).Shrink(0).FontSize(12.5).FontWeight(600).TextColor(secondary(t))
				if a.Detail != "" {
					components.Text(c, a.Detail).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5).TextColor(secondary(t))
				} else {
					ui.Box(c).Grow(1)
				}
				switch {
				case a.Status == Failed:
					components.Icon(c, lucide.X).Size(10, 10).Shrink(0).TextColor(t.Danger)
				case a.Status == Running && !detail:
					pulseDot(c, t.Accent)
				}
				if detail {
					icon := lucide.ChevronRight
					if open {
						icon = lucide.ChevronDown
					}
					components.Icon(c, icon).Size(10, 10).Shrink(0).TextColor(tertiary(t))
				}
			})
		}
		if detail {
			b := ui.ButtonBase(c).Label(a.Action + " " + a.Detail).Expanded(open).Justify(ui.Start).Background(ui.Color{}).TextColor(t.Text)
			if b.Hovered() || b.Focused() {
				b.Background(t.Text.Alpha(.03))
			}
			header(b)
			if b.Clicked() {
				a.State.Expanded = !a.State.Expanded
				open = a.State.Expanded
				pauseConversation(c)
			}
		} else {
			header(ui.Row(c))
		}
		if !open {
			return
		}
		height := a.MaxHeight
		if height <= 0 {
			height = 400
		}
		if a.Status == Running && a.State.Scroll.Y >= a.State.Scroll.MaxY-2 {
			a.State.Scroll.Y = math.MaxFloat32
		}
		components.ScrollView(c, components.ScrollViewOptions{}).FillWidth().MaxHeight(height).BorderWidth(1, 0, 0, 0).BorderColor(strongBorder(t)).
			TrackScroll(&a.State.Scroll).Children(func() {
			body := ui.Column(c).FillWidth().MinWidth(0).Padding(8, 12).Gap(8).TextColor(secondary(t))
			if a.Mono {
				body.Font("monospace").FontSize(12)
			}
			body.Children(func() { a.Content(c) })
		})
	})
}

func pulseDot(c *ui.Context, color ui.Color) {
	reduced := c.Preferences().ReduceMotion
	ui.Box(c).Size(5, 5).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		alpha := float32(1)
		if !reduced {
			p.After(50 * time.Millisecond)
			alpha = float32(.45 + .55*(1+math.Sin(float64(p.Now().UnixMilli()%1000)/1000*2*math.Pi))/2)
		}
		p.Fill(r, color.Alpha(alpha), r.W/2)
	})
}

func divider(c *ui.Context) { ui.Box(c).Grow(1).Height(1).Background(c.Theme().Border) }

// Waku-derived tones layered on the frame theme.
func secondary(t *ui.Theme) ui.Color    { return t.Text.Mix(t.TextMuted, .45) }
func tertiary(t *ui.Theme) ui.Color     { return t.TextMuted }
func strongBorder(t *ui.Theme) ui.Color { return t.Border.Mix(t.Text, .08) }
func overlay(t *ui.Theme) ui.Color      { return t.Text.Alpha(.05) }
func overlayStrong(t *ui.Theme) ui.Color {
	return t.Text.Alpha(.09)
}

func pauseConversation(c *ui.Context) {
	scope := *conversationSlot(c)
	if scope.state != nil {
		scope.state.Scroll.Y = scope.previousY
		scope.state.paused, scope.state.pauseY = true, scope.previousY
	}
}

// Message lays out a message independently of its surface. Outgoing messages
// align to the trailing edge; incoming content uses the available width.
func Message(c *ui.Context, outgoing bool, content View) ui.Element {
	e := ui.Column(c).FillWidth().MinWidth(0).Shrink(0)
	if outgoing {
		e.AlignItems(ui.End)
	}
	return e.Children(func() {
		if content != nil {
			content(c)
		}
	})
}

// Bubble is the raised surface of an outgoing message.
func Bubble(c *ui.Context, content View) ui.Element {
	t := c.Theme()
	return ui.Column(c).MinWidth(0).MaxWidth(540).Padding(9, 14, 8, 14).Gap(7).Radius(12).
		Background(t.Background.Mix(t.Text, .06)).Children(func() {
		if content != nil {
			content(c)
		}
	})
}

// ConversationState retains MyGo's scroll state. Use one per transcript;
// replacing a transcript should also replace this state. This first version
// uses ordinary scrolling to preserve selection; it does not virtualize history
// or anchor prepended rows. Applications must bound retained history themselves.
type ConversationState struct {
	Scroll ui.ScrollState
	paused bool
	pauseY float32
}

type (
	conversationKey   struct{}
	conversationScope struct {
		state     *ConversationState
		previousY float32
		reading   bool
	}
)

func conversationSlot(c *ui.Context) *conversationScope {
	return ui.Local(c.Root(), conversationKey{}, func() conversationScope { return conversationScope{} })
}

// AtLatest reports whether the previously laid-out viewport is at the bottom.
func (s *ConversationState) AtLatest() bool { return s.Scroll.Y >= s.Scroll.MaxY-2 }

// JumpToLatest resumes bottom following on the next build.
func (s *ConversationState) JumpToLatest() {
	s.paused = false
	s.Scroll.Y = math.MaxFloat32
}

// ContentWidth is the reading width of transcripts and the composer.
const ContentWidth = 720

// Conversation owns a bounded scroll viewport and a jump button. Give its
// parent a bounded height. Supply a localized jumpLabel. Following uses MyGo's
// scroll state and yields when the user scrolls up. The button is overlaid so
// its appearance cannot change viewport height.
func Conversation(c *ui.Context, state *ConversationState, jumpLabel string, content View) ui.Element {
	return transcript(c, state, jumpLabel, 22, 20, 48, content)
}

// transcript is Conversation with its content padding and the gap between
// turns, which narrow hosts such as Sidebar tighten.
func transcript(c *ui.Context, state *ConversationState, jumpLabel string, padY, padX, gap float32, content View) ui.Element {
	if state.paused && state.Scroll.Y != state.pauseY && state.AtLatest() {
		state.paused = false
	}
	follow := state.AtLatest() && !state.paused
	previousY := state.Scroll.Y
	if follow {
		state.JumpToLatest()
	}
	return ui.Column(c).FillWidth().Grow(1).MinHeight(0).MinWidth(0).Children(func() {
		viewport := components.ScrollView(c, components.ScrollViewOptions{}).Key("viewport").FillWidth().Grow(1).MinHeight(0).TrackScroll(&state.Scroll)
		viewport.Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Shrink(0).AlignItems(ui.Center).Padding(padY, padX).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(ContentWidth).MinWidth(0).Gap(gap).Children(func() {
					slot := conversationSlot(c)
					saved := *slot
					*slot = conversationScope{state: state, previousY: previousY, reading: !follow}
					defer func() { *slot = saved }()
					if content != nil {
						content(c)
					}
				})
			})
		})
		// A disclosure can cancel this frame's follow without moving its header.
		viewport.TrackScroll(&state.Scroll)
		if !follow {
			b := components.Button(c, "", components.ButtonSecondary).Label(jumpLabel).Absolute().Attach(ui.AnchorBottom, ui.AnchorBottom).
				Margin(c.Theme().Space(2)).Size(28, 28).Padding(0).Radius(14)
			b.Children(func() { components.Icon(c, lucide.ArrowDown).Size(14, 14) })
			components.Tooltip(c, b, jumpLabel)
			if b.Clicked() {
				state.JumpToLatest()
				c.Invalidate()
			}
		}
	})
}

// ComposerOptions supplies localized labels and optional application controls.
// Enter sends outside input-method composition and Shift+Enter inserts a
// newline. While Running, Stop is shown, and a nonempty draft also offers
// Send so the application can interrupt the run with the new message.
type ComposerOptions struct {
	Label, Placeholder, SendLabel, StopLabel string
	Running                                  bool
	// Accessory holds controls in the card's bottom row, such as model chips.
	Accessory View
	// Trailing holds compact controls immediately before Send or Stop, with
	// an 8-point gap. Unlike Accessory, this slot does not shrink.
	Trailing View
	// Attachments appears above the editor; Footer below the card.
	Attachments, Footer View
	// AcceptFiles makes the card a drop target for files from other apps.
	AcceptFiles bool
	// DropOptions additionally accepts native data such as images. The host
	// prepares the returned Drop; the composer never reads files or stores data.
	DropOptions transfer.DropOptions
	// OnPaste runs for keyboard and menu paste, including non-text clipboard
	// contents. Return true to consume it; false preserves normal text editing.
	// Hosts may read the clipboard here, but must prepare attachments off-thread.
	OnPaste func(text string) bool
	// SendDisabled prevents sending while the host prepares attachments or is
	// otherwise not ready. Editing and Stop remain available.
	SendDisabled bool
	// StopDisabled keeps Stop shown but inactive while the host's run cannot
	// be interrupted yet. Editing and Send remain available.
	StopDisabled bool
	// Keyboard, Capitalize and Autofill describe the editor to the on-screen
	// keyboards of phones and tablets, as TextBox's same-named options do.
	// ReturnKey names what the keyboard's return key says and defaults to
	// Send, as plain Enter sends; Autocorrect turns the keyboard's
	// corrections and suggestions on or off, nil leaving them to the
	// keyboard.
	Keyboard    ui.KeyboardType
	ReturnKey   ui.ReturnKeyType
	Capitalize  ui.Capitalization
	Autofill    ui.AutofillKind
	Autocorrect *bool
}

// composerEditorOptions maps the composer's keyboard options onto its
// editor's.
func composerEditorOptions(options ComposerOptions) components.TextBoxOptions {
	editorOptions := components.TextBoxOptions{
		Multiline:   true,
		Keyboard:    options.Keyboard,
		ReturnKey:   options.ReturnKey,
		Capitalize:  options.Capitalize,
		Autofill:    options.Autofill,
		Autocorrect: options.Autocorrect,
	}
	if editorOptions.ReturnKey == ui.ReturnDefault {
		editorOptions.ReturnKey = ui.ReturnSend
	}
	return editorOptions
}

type composerEnterKey struct{}

// ComposerResult contains frame-local actions. The application clears the
// draft after accepting Send and owns request cancellation for Stop. Dropped
// lists file paths dropped on the card since the last frame; reading or
// attaching them remains application work. Drop contains host-selected native
// data, while Dragging reports either kind of accepted drag over the card.
type ComposerResult struct {
	Root, Input ui.Element
	// Card is the editor surface, excluding Footer. Hosts may style its
	// background or material without changing the frame theme.
	Card       ui.Element
	Send, Stop bool
	Dropped    []string
	Drop       *transfer.Drop
	Dragging   bool
}

// Composer builds a multiline editor card with send/stop and an accessory row.
// Keep draft outside the frame; a running request never discards the next draft.
func Composer(c *ui.Context, draft *string, options ComposerOptions) ComposerResult {
	var result ComposerResult
	t := c.Theme()
	empty := strings.TrimSpace(*draft) == ""
	canSend := !empty && !options.SendDisabled
	result.Root = ui.Column(c).FillWidth().MinWidth(0).Shrink(0).Children(func() {
		card := ui.Column(c).FillWidth().MinWidth(0).Padding(10, 0).Radius(13).Border(1, t.Border).Background(t.Surface).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, .04)).Children(func() {
			if options.Attachments != nil {
				ui.Row(c).Key("attachments").FillWidth().Wrap().Gap(6).Padding(0, 10, 6, 10).Children(func() { options.Attachments(c) })
			}
			result.Input = components.TextBox(c.Key("input"), draft, composerEditorOptions(options)).FillWidth().Lines(1, 10).MinHeight(40).
				Label(options.Label).Placeholder(options.Placeholder).Border(0, ui.Color{}).Background(ui.Color{}).Padding(2, 14, 0, 14).FontSize(13.5)
			// MyGo's OnPaste wraps fn without a nil check, so attach only a real hook.
			if options.OnPaste != nil {
				result.Input.OnPaste(options.OnPaste)
			}
			ui.Row(c).FillWidth().MinWidth(0).Margin(8, 0, 0, 0).Padding(0, 10).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(4).Children(func() {
					if options.Accessory != nil {
						options.Accessory(c)
					}
				})
				if options.Trailing != nil {
					ui.Row(c).Key("trailing").Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() { options.Trailing(c) })
				}
				if options.Running && empty {
					background, foreground := t.Danger.Alpha(.14), t.Danger
					if options.StopDisabled {
						background, foreground = overlayStrong(t), tertiary(t)
					}
					ui.Box(c).Disabled(options.StopDisabled).Children(func() {
						result.Stop = pillAction(c, options.StopLabel, background, foreground).Clicked()
					})
				}
				if !options.Running || !empty {
					background, foreground := t.Accent, t.AccentText
					if !canSend {
						background, foreground = overlayStrong(t), tertiary(t)
					}
					ui.Box(c).Disabled(!canSend).Children(func() {
						if pillAction(c, options.SendLabel, background, foreground).Clicked() {
							result.Send = true
						}
					})
				}
			})
		})
		result.Card = card
		// The multiline editor inserts a newline for Enter before shortcuts
		// see it, so take plain Enter as it arrives. Composition keeps its own
		// Enter; Shift+Enter still reaches the editor.
		pending := ui.Local(result.Input, composerEnterKey{}, func() bool { return false })
		composing := result.Input.Composing()
		result.Input.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputKeyDown || ev.Key != ui.KeyEnter || ev.Mods != 0 || composing {
				return false
			}
			*pending = true
			return true
		})
		if *pending {
			*pending = false
			result.Send = result.Send || canSend && !composing
		}
		if options.AcceptFiles {
			result.Dropped = card.DroppedFiles()
			result.Dragging = card.FileDragOver()
		}
		if len(options.DropOptions.Formats) > 0 {
			if drop, ok := ui.DropData(card, options.DropOptions); ok {
				result.Drop = &drop
			}
			_, over := ui.DataDragOver(card, options.DropOptions)
			result.Dragging = result.Dragging || over
		}
		if result.Dragging {
			card.BorderColor(t.Accent.Alpha(.7)).Background(t.Surface.Mix(t.Text, .09))
		} else if result.Input.Focused() {
			card.BorderColor(strongBorder(t))
		}
		if options.Footer != nil {
			// The 28-DIP strip holds a fine pointer's compact controls; a
			// coarse one's TouchMin-sized controls raise it.
			h := max(float32(28), components.TouchMin(c)+4)
			ui.Row(c).Key("composer-footer").FillWidth().MinWidth(0).Height(h).Padding(4, 10, 0, 10).AlignItems(ui.Center).Gap(2).
				Children(func() { options.Footer(c) })
		}
	})
	return result
}

func pillAction(c *ui.Context, label string, background, foreground ui.Color) ui.Element {
	h := max(float32(26), components.TouchMin(c))
	b := ui.ButtonBase(c).Label(label).Height(h).Padding(0, 12).Shrink(0).Radius(h / 2).Center().Background(background).TextColor(foreground)
	if b.Pressed() {
		b.Opacity(.8)
	} else if b.Hovered() {
		b.Opacity(.9)
	}
	b.Children(func() { components.Text(c, label).SingleLine().FontSize(12.5).FontWeight(600).TextColor(foreground) })
	return b
}

// IconAction is a keyboard-operable, named icon button with a focus tooltip.
func IconAction(c *ui.Context, label string, icon func() *ui.SVG) ui.Element {
	t := c.Theme()
	hit := max(float32(26), components.TouchMin(c))
	b := ui.ButtonBase(c).Label(label).Size(hit, hit).Shrink(0).Radius(6).Center().Background(ui.Color{}).TextColor(tertiary(t))
	if b.Hovered() || b.Focused() {
		b.Background(overlay(t)).TextColor(t.Text)
	}
	b.Children(func() { components.Icon(c, icon).Size(14, 14) })
	components.Tooltip(c, b, label)
	return b
}
