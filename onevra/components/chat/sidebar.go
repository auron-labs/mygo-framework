package chat

import (
	"slices"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/internal/l10n"
	"github.com/egoist/mygo/ui"
)

// SidebarState retains a Sidebar's draft and presentation. Keep one per
// sidebar for its lifetime: it resets the transcript's scroll and the turns'
// folding itself when SidebarOptions.Conversation changes.
type SidebarState struct {
	// Draft is the composer's text. Clear it after accepting Send.
	Draft string
	// History shows the conversation list in place of the transcript. The
	// header's History button toggles it; choosing a conversation or New
	// closes it.
	History bool
	// Agent, Model and Effort retain the pickers' popovers.
	Agent, Model PickerState
	Effort       EffortState

	conversation string
	asking       string // the permission the card last showed
	transcript   ConversationState
	history      ui.ScrollState
	turns        map[string]*TurnState
	groups       map[string]*GroupState
	activities   map[string]*ActivityState
}

// Turn builds a turn with the sidebar's retained folding, filling each
// step's nil Group and each detailed activity's nil State by ID. Use it to
// build turns in SidebarOptions.Transcript.
func (s *SidebarState) Turn(c *ui.Context, options TurnOptions) ui.Element {
	// Copies keep the application's slices untouched by the filled states.
	options.Steps = slices.Clone(options.Steps)
	for i := range options.Steps {
		step := &options.Steps[i]
		if len(step.Activities) == 0 {
			continue
		}
		if step.Group == nil {
			step.Group = retained(&s.groups, options.ID+"\x00"+step.ID)
		}
		step.Activities = slices.Clone(step.Activities)
		for j := range step.Activities {
			if a := &step.Activities[j]; a.Content != nil && a.State == nil {
				a.State = retained(&s.activities, options.ID+"\x00"+a.ID)
			}
		}
	}
	return Turn(c, retained(&s.turns, options.ID), options)
}

func retained[T any](states *map[string]*T, key string) *T {
	if *states == nil {
		*states = map[string]*T{}
	}
	state := (*states)[key]
	if state == nil {
		state = new(T)
		(*states)[key] = state
	}
	return state
}

// Suggestion starts a conversation: choosing it puts Prompt, or Label when
// Prompt is empty, in the composer.
type Suggestion struct {
	Label, Prompt string
	Icon          func() *ui.SVG
}

// Permission is a question a running turn waits on, such as whether the
// agent may run a command. Detail shows in monospace, as a command or path
// does; Content replaces it, for example with a diff. Options are the
// answers in order, the first being the primary one; only their ID and
// Label are used.
type Permission struct {
	ID, Title, Detail string
	Content           View
	Options           []Option
}

// PermissionAnswer is the option chosen for a permission; both IDs are empty
// when none was.
type PermissionAnswer struct{ Permission, Option string }

// SidebarOptions describes the shown conversation and what surrounds it.
// Strings are application content. The sidebar names its own controls from
// Onevra's catalog in the window's language; applications change them with
// "onevra.chat…" messages in their catalogs.
type SidebarOptions struct {
	// Conversation is the shown conversation's stable ID. Changing it resets
	// the transcript's scroll and the turns' folding.
	Conversation string
	// Title names the conversation in the header; empty reads
	// "New conversation".
	Title string
	// Turns are the conversation's requests and responses, oldest first,
	// built as SidebarState.Turn builds them.
	Turns []TurnOptions
	// Transcript replaces the built Turns and the empty state, for content
	// such as turns interleaved with notices. Build its turns with
	// SidebarState.Turn to keep their folding.
	Transcript View
	// Running shows Stop in the composer and disables New, History and the
	// pickers until the turn ends.
	Running bool
	// Permissions are the questions the running turn waits on, oldest first.
	// The first shows as a card at the end of the transcript, scrolled into
	// view when it arrives.
	Permissions []Permission

	// EmptyIcon (Sparkles by default), EmptyTitle and EmptyText introduce a
	// conversation without turns, above Suggestions. Empty replaces all four.
	EmptyIcon             func() *ui.SVG
	EmptyTitle, EmptyText string
	Suggestions           []Suggestion
	Empty                 View

	// Conversations are what History lists, newest first: Label is a
	// conversation's title, Detail trailing text such as when it was last
	// updated, and Group a heading such as "Today" over consecutive ones.
	// Nil omits History.
	Conversations []Option
	// Deletable gives each listed conversation a delete button.
	Deletable bool

	// Agents fill the picker below the composer, and Models and Efforts the
	// model and reasoning pickers in it; Agent, Model and Effort are the
	// chosen IDs. Empty lists omit their pickers.
	Agents  []Option
	Agent   string
	Models  []Option
	Model   string
	Efforts []Option
	Effort  string
	// RefreshAgents shows a button beside the agent picker that asks to
	// look for agents again.
	RefreshAgents bool

	// Notice is short status above the composer, such as while the
	// application looks for agents, or why a request cannot be sent.
	Notice string
	// Composer is passed to the composer. The sidebar sets Running, fills
	// empty labels and the placeholder, adds the model and reasoning pickers
	// after Accessory, and the agent picker before Footer.
	Composer ComposerOptions
	// HeaderActions builds controls in the header before History and New,
	// such as one that closes the sidebar. Top builds content between the
	// header and the transcript, such as what the conversation works on.
	HeaderActions, Top View
	// TopInset reserves room above the header for a title bar the sidebar
	// sits under, such as sidebar.HeaderHeight(c) in a sidebar shell's Right.
	TopInset float32
}

// SidebarResult reports frame-local actions; the application applies them.
type SidebarResult struct {
	Root ui.Element
	// Composer reports Send, Stop, dropped files and the composer's elements.
	Composer ComposerResult
	// New asks for a new conversation; Open and Delete name one from History.
	New          bool
	Open, Delete string
	// Agent, Model and Effort are IDs chosen from the pickers.
	Agent, Model, Effort string
	// RefreshAgents asks to look for agents again.
	RefreshAgents bool
	Answer        PermissionAnswer
}

// Sidebar is an assistant panel: a header with the conversation's title,
// History and New; the transcript, or an empty state with suggestions, and
// the permission the running turn waits on; and the composer with agent,
// model and reasoning pickers. Give its parent a bounded height. It starts
// no work: the application runs turns, keeps conversations and applies the
// result.
func Sidebar(c *ui.Context, state *SidebarState, options SidebarOptions) SidebarResult {
	var result SidebarResult
	words := l10n.In(c.Locale())
	if state.conversation != options.Conversation {
		state.conversation, state.transcript = options.Conversation, ConversationState{}
		state.turns, state.groups, state.activities = nil, nil, nil
	}
	if options.Conversations == nil {
		state.History = false
	}
	if options.Running {
		state.Agent.Open, state.Model.Open, state.Effort.Open = false, false, false
	}
	asking := ""
	if len(options.Permissions) > 0 {
		asking = options.Permissions[0].ID
	}
	if asking != "" && asking != state.asking {
		state.transcript.JumpToLatest()
	}
	state.asking = asking
	suggested := false
	result.Root = ui.Column(c).FillWidth().Grow(1).MinHeight(0).MinWidth(0).Padding(max(0, options.TopInset), 0, 0, 0).Children(func() {
		sidebarHeader(c, words, state, options, &result)
		if options.Top != nil {
			ui.Column(c).Key("top").FillWidth().MinWidth(0).Shrink(0).Padding(0, 12, 4, 12).Children(func() { options.Top(c) })
		}
		if state.History {
			conversationList(c, words, state, options, &result)
		} else {
			transcript(c, &state.transcript, words.Text("chatJumpToLatest"), 12, 14, 28, func(c *ui.Context) {
				switch {
				case options.Transcript != nil:
					options.Transcript(c)
				case len(options.Turns) == 0:
					suggested = emptyState(c, state, options)
				default:
					for _, turn := range options.Turns {
						state.Turn(c, turn)
					}
				}
				if len(options.Permissions) > 0 {
					result.Answer = permissionCard(c, words, options.Permissions)
				}
			})
		}
		ui.Column(c).Key("composer").FillWidth().MinWidth(0).Shrink(0).Padding(4, 10, 10, 10).Gap(8).Children(func() {
			if options.Notice != "" {
				components.Text(c, options.Notice).Wrap().Padding(0, 4).FontSize(12).TextColor(tertiary(c.Theme()))
			}
			result.Composer = Composer(c, &state.Draft, sidebarComposer(words, state, options, &result))
		})
	})
	if suggested {
		result.Composer.Input.Focus()
	}
	return result
}

func sidebarHeader(c *ui.Context, words l10n.Messages, state *SidebarState, options SidebarOptions, result *SidebarResult) {
	t := c.Theme()
	ui.Row(c).Key("header").FillWidth().MinWidth(0).Shrink(0).Height(max(float32(40), components.TouchMin(c)+8)).Padding(0, 8, 0, 14).
		AlignItems(ui.Center).Gap(2).Children(func() {
		title := options.Title
		switch {
		case state.History:
			title = words.Text("chatConversations")
		case title == "":
			title = words.Text("chatNewConversation")
		}
		components.Text(c, title).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(13).FontWeight(600).TextColor(t.Text)
		if options.HeaderActions != nil {
			ui.Row(c).Key("actions").Shrink(0).AlignItems(ui.Center).Gap(2).Children(func() { options.HeaderActions(c) })
		}
		if options.Conversations != nil {
			history := IconAction(c, words.Text("chatHistory"), lucide.Clock).Checked(state.History).Disabled(options.Running)
			if state.History {
				history.Background(overlayStrong(t)).TextColor(t.Text)
			}
			if history.Clicked() {
				state.History = !state.History
			}
		}
		if IconAction(c, words.Text("chatNewConversation"), lucide.SquarePen).Disabled(options.Running).Clicked() {
			state.History, result.New = false, true
		}
	})
}

func conversationList(c *ui.Context, words l10n.Messages, state *SidebarState, options SidebarOptions, result *SidebarResult) {
	t := c.Theme()
	components.ScrollView(c, components.ScrollViewOptions{}).Key("history").FillWidth().Grow(1).MinHeight(0).TrackScroll(&state.history).Children(func() {
		ui.Column(c).FillWidth().MinWidth(0).Padding(4, 8, 12, 8).Gap(1).Children(func() {
			if len(options.Conversations) == 0 {
				components.Text(c, words.Text("chatNoConversations")).Wrap().Padding(8).FontSize(12.5).TextColor(tertiary(t))
				return
			}
			group := ""
			for i, o := range options.Conversations {
				if o.Group != "" && (i == 0 || o.Group != group) {
					components.Text(c, o.Group).Padding(10, 8, 3, 8).FontSize(11.5).FontWeight(500).TextColor(tertiary(t))
				}
				group = o.Group
				ui.Row(c).Key(o.ID).FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(2).Children(func() {
					current := o.ID == options.Conversation
					b := ui.ButtonBase(c).Label(o.Label).Selected(current).Grow(1).MinWidth(0).Height(max(float32(32), components.TouchMin(c))).
						Padding(0, 8).Radius(6).Justify(ui.Start).AlignItems(ui.Center).Gap(8).Background(ui.Color{}).TextColor(t.Text)
					if current || b.Hovered() || b.Focused() {
						b.Background(overlay(t))
					}
					b.Children(func() {
						color, weight := secondary(t), 400
						if current {
							color, weight = t.Text, 500
						}
						components.Text(c, o.Label).Grow(1).MinWidth(0).SingleLine().Ellipsis("…").FontSize(13).FontWeight(weight).TextColor(color)
						if o.Detail != "" {
							components.Text(c, o.Detail).Shrink(0).FontSize(11.5).TextColor(tertiary(t))
						}
					})
					if b.Clicked() {
						state.History, result.Open = false, o.ID
					}
					if options.Deletable && IconAction(c, words.Label("chatDeleteConversation", o.Label), lucide.Trash).Clicked() {
						result.Delete = o.ID
					}
				})
			}
		})
	})
}

// emptyState introduces a conversation without turns and reports whether a
// suggestion filled the draft.
func emptyState(c *ui.Context, state *SidebarState, options SidebarOptions) bool {
	if options.Empty != nil {
		options.Empty(c)
		return false
	}
	t := c.Theme()
	chosen := false
	ui.Column(c).Key("empty").FillWidth().MinWidth(0).AlignItems(ui.Center).Gap(8).Padding(20, 4, 0, 4).Children(func() {
		icon := options.EmptyIcon
		if icon == nil {
			icon = lucide.Sparkles
		}
		ui.Row(c).Size(40, 40).Shrink(0).Radius(10).Center().Background(overlay(t)).Children(func() {
			components.Icon(c, icon).Size(20, 20).TextColor(t.Accent)
		})
		if options.EmptyTitle != "" {
			components.Text(c, options.EmptyTitle).Wrap().TextAlign(ui.Center).FontSize(14).FontWeight(600).TextColor(t.Text)
		}
		if options.EmptyText != "" {
			components.Text(c, options.EmptyText).Wrap().TextAlign(ui.Center).FontSize(12.5).TextColor(tertiary(t))
		}
		if len(options.Suggestions) == 0 {
			return
		}
		ui.Column(c).FillWidth().MinWidth(0).Margin(8, 0, 0, 0).Gap(6).Children(func() {
			for _, s := range options.Suggestions {
				b := ui.ButtonBase(c).Label(s.Label).FillWidth().MinWidth(0).Height(max(float32(32), components.TouchMin(c))).Padding(0, 10).
					Radius(8).Border(1, strongBorder(t)).Justify(ui.Start).AlignItems(ui.Center).Gap(8).Background(ui.Color{}).TextColor(t.Text)
				if b.Hovered() || b.Focused() {
					b.Background(overlay(t))
				}
				b.Children(func() {
					if s.Icon != nil {
						components.Icon(c, s.Icon).Size(14, 14).Shrink(0).TextColor(tertiary(t))
					}
					components.Text(c, s.Label).MinWidth(0).SingleLine().Ellipsis("…").FontSize(13).TextColor(secondary(t))
				})
				if b.Clicked() {
					state.Draft, chosen = s.Prompt, true
					if state.Draft == "" {
						state.Draft = s.Label
					}
				}
			}
		})
	})
	return chosen
}

func permissionCard(c *ui.Context, words l10n.Messages, permissions []Permission) PermissionAnswer {
	t := c.Theme()
	p := permissions[0]
	var answer PermissionAnswer
	ui.Column(c).Key("permission-"+p.ID).FillWidth().MinWidth(0).Shrink(0).Padding(12).Radius(10).Border(1, strongBorder(t)).Background(t.Surface).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			components.Icon(c, lucide.ShieldAlert).Size(13, 13).Shrink(0).TextColor(t.Warning)
			components.Text(c, words.Text("chatNeedsApproval")).FontSize(11.5).FontWeight(600).TextColor(t.Warning)
		})
		ui.Column(c).FillWidth().MinWidth(0).Gap(4).Children(func() {
			components.Text(c, p.Title).Wrap().FontSize(13).FontWeight(600).TextColor(t.Text)
			switch {
			case p.Content != nil:
				p.Content(c)
			case p.Detail != "":
				components.Text(c, p.Detail).Wrap().Selectable().Font("monospace").FontSize(12).TextColor(secondary(t))
			}
		})
		ui.Row(c).FillWidth().MinWidth(0).Wrap().Gap(6).Children(func() {
			for i, o := range p.Options {
				variant := components.ButtonSecondary
				if i == 0 {
					variant = components.ButtonPrimary
				}
				if components.Button(c, o.Label, variant).Clicked() {
					answer = PermissionAnswer{Permission: p.ID, Option: o.ID}
				}
			}
		})
		if waiting := len(permissions) - 1; waiting > 0 {
			components.Text(c, words.Count("chatMoreWaiting", waiting)).FontSize(12).TextColor(tertiary(t))
		}
	})
	return answer
}

// sidebarComposer adds the sidebar's labels and pickers to the application's
// composer options.
func sidebarComposer(words l10n.Messages, state *SidebarState, options SidebarOptions, result *SidebarResult) ComposerOptions {
	composer := options.Composer
	composer.Running = options.Running
	for _, label := range []struct {
		value *string
		key   string
	}{
		{&composer.Label, "chatMessage"},
		{&composer.Placeholder, "chatPlaceholder"},
		{&composer.SendLabel, "chatSend"},
		{&composer.StopLabel, "chatStop"},
	} {
		if *label.value == "" {
			*label.value = words.Text(label.key)
		}
	}
	if accessory := composer.Accessory; len(options.Models) > 0 || len(options.Efforts) > 0 {
		composer.Accessory = func(c *ui.Context) {
			if accessory != nil {
				accessory(c)
			}
			ui.Row(c).Key("pickers").MinWidth(0).Shrink(1).AlignItems(ui.Center).Gap(2).Disabled(options.Running).Children(func() {
				if len(options.Models) > 0 {
					result.Model = Picker(c, &state.Model, PickerOptions{
						Label:       optionLabel(options.Models, options.Model, words.Text("chatModel")),
						SearchLabel: words.Text("chatSearchModels"), EmptyText: words.Text("chatNoMatches"),
						Options: options.Models, Selected: options.Model,
					}).Chosen
				}
				if len(options.Efforts) > 0 {
					result.Effort = EffortPicker(c, &state.Effort, EffortPickerOptions{
						Label: words.Text("chatEffort"), Icon: lucide.Brain, Levels: options.Efforts, Selected: options.Effort,
					}).Chosen
				}
			})
		}
	}
	if footer := composer.Footer; len(options.Agents) > 0 || options.RefreshAgents {
		composer.Footer = func(c *ui.Context) {
			ui.Row(c).Key("agents").MinWidth(0).Shrink(1).AlignItems(ui.Center).Gap(2).Disabled(options.Running).Children(func() {
				ui.Row(c).MinWidth(0).Shrink(1).Disabled(len(options.Agents) == 0).Children(func() {
					result.Agent = Picker(c, &state.Agent, PickerOptions{
						Label: optionLabel(options.Agents, options.Agent, words.Text("chatChooseAgent")), Icon: lucide.Bot,
						SearchLabel: words.Text("chatSearchAgents"), EmptyText: words.Text("chatNoMatches"),
						Options: options.Agents, Selected: options.Agent,
					}).Chosen
				})
				if options.RefreshAgents && IconAction(c, words.Text("chatRefreshAgents"), lucide.RotateCw).Clicked() {
					result.RefreshAgents = true
				}
			})
			if footer != nil {
				footer(c)
			}
		}
	}
	return composer
}

// optionLabel is the label of the chosen option, or fallback when none is.
func optionLabel(options []Option, id, fallback string) string {
	for _, o := range options {
		if o.ID == id {
			return o.Label
		}
	}
	return fallback
}
