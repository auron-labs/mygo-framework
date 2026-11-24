package chat

import (
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// collect keeps every action a sidebar reported across the frames a test
// interaction spans.
func collect(got *SidebarResult, r SidebarResult) {
	got.Composer.Send = got.Composer.Send || r.Composer.Send
	got.Composer.Stop = got.Composer.Stop || r.Composer.Stop
	got.New = got.New || r.New
	got.RefreshAgents = got.RefreshAgents || r.RefreshAgents
	for _, field := range []struct{ to, from *string }{
		{&got.Open, &r.Open}, {&got.Delete, &r.Delete}, {&got.Agent, &r.Agent}, {&got.Model, &r.Model}, {&got.Effort, &r.Effort},
	} {
		if *field.from != "" {
			*field.to = *field.from
		}
	}
	if r.Answer != (PermissionAnswer{}) {
		got.Answer = r.Answer
	}
}

func text(s string) View { return func(c *ui.Context) { components.Text(c, s) } }

func TestSidebarRetainsTurnFoldingPerConversation(t *testing.T) {
	var state SidebarState
	opts := SidebarOptions{Conversation: "one", Turns: []TurnOptions{{
		ID: "turn", Status: Completed, Summary: "Worked", Final: text("Answer"),
		Steps: []Step{{ID: "step", Title: "Ran 1 command", Activities: []Activity{{ID: "call", Action: "Run", Detail: "ls", Content: text("output")}}}},
	}}}
	tt := ui.NewTester(func(c *ui.Context) { Sidebar(c, &state, opts) }, 360, 640)
	if !tt.HasText("Answer") || tt.HasText("Ran 1 command") {
		t.Fatal("completed turn did not fold its steps")
	}
	for _, label := range []string{"Worked", "Ran 1 command", "Run ls"} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	tt.Frame()
	if !tt.HasText("output") {
		t.Fatal("the step group or activity did not keep its expansion")
	}
	if opts.Turns[0].Steps[0].Group != nil || opts.Turns[0].Steps[0].Activities[0].State != nil {
		t.Fatal("retained state was written into the application's turns")
	}
	opts.Conversation = "two"
	tt.Frame()
	if tt.HasText("Ran 1 command") {
		t.Fatal("another conversation inherited the turn's expansion")
	}
}

func TestSidebarSuggestionsHistoryAndNew(t *testing.T) {
	var state SidebarState
	var got SidebarResult
	opts := SidebarOptions{
		Conversation: "one", Title: "First chat", EmptyTitle: "Ask about this project",
		Suggestions:   []Suggestion{{Label: "Summarize", Prompt: "Summarize this file"}},
		Conversations: []Option{{ID: "one", Label: "First chat", Detail: "2h"}, {ID: "two", Label: "Second chat"}},
		Deletable:     true,
	}
	tt := ui.NewTester(func(c *ui.Context) { collect(&got, Sidebar(c, &state, opts)) }, 360, 640)
	if err := tt.Click("Summarize"); err != nil {
		t.Fatal(err)
	}
	if state.Draft != "Summarize this file" || !tt.Focused("Message") {
		t.Fatalf("suggestion did not fill and focus the composer: %q", state.Draft)
	}
	tt.Key(0, ui.KeyEnter)
	if !got.Composer.Send {
		t.Fatal("Enter did not send the suggested draft")
	}
	if err := tt.Click("Conversation history"); err != nil {
		t.Fatal(err)
	}
	if info, _ := tt.Access("Conversation history"); !state.History || !info.Checked || !tt.HasText("Second chat") {
		t.Fatal("history did not open in place of the transcript")
	}
	if err := tt.Click("Delete Second chat"); err != nil || got.Delete != "two" {
		t.Fatalf("delete: %v %q", err, got.Delete)
	}
	if err := tt.Click("Second chat"); err != nil || got.Open != "two" || state.History {
		t.Fatalf("open: %v %q history=%v", err, got.Open, state.History)
	}
	if err := tt.Click("New conversation"); err != nil || !got.New {
		t.Fatalf("new: %v", err)
	}
	opts.Conversations = nil
	tt.Frame()
	if _, ok := tt.Access("Conversation history"); ok {
		t.Fatal("History shown without conversations to list")
	}
}

func TestSidebarPermissionsWhileRunning(t *testing.T) {
	var state SidebarState
	var got SidebarResult
	answers := []Option{{ID: "allow", Label: "Allow once"}, {ID: "deny", Label: "Deny"}}
	opts := SidebarOptions{
		Conversation: "one", Title: "Tests", Running: true, Conversations: []Option{},
		Agents: []Option{{ID: "a", Label: "Claude Code"}}, Agent: "a",
		Turns: []TurnOptions{{ID: "turn", Status: Running, Prompt: text("Run the tests")}},
		Permissions: []Permission{
			{ID: "p1", Title: "Run npm test", Detail: "npm test", Options: answers},
			{ID: "p2", Title: "Edit package.json", Options: answers},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { collect(&got, Sidebar(c, &state, opts)) }, 360, 640)
	if !tt.HasText("Run npm test") || tt.HasText("Edit package.json") || !tt.HasText("1 more request waiting") {
		t.Fatal("the first permission and the waiting count are not shown")
	}
	if err := tt.Click("Deny"); err != nil || got.Answer != (PermissionAnswer{Permission: "p1", Option: "deny"}) {
		t.Fatalf("answer: %v %+v", err, got.Answer)
	}
	for _, name := range []string{"New conversation", "Conversation history", "Claude Code"} {
		if info, ok := tt.Access(name); !ok || !info.Disabled {
			t.Fatalf("%s is not disabled while running", name)
		}
	}
	if err := tt.Click("Stop"); err != nil || !got.Composer.Stop {
		t.Fatalf("stop: %v", err)
	}
}

func TestSidebarPickersKeepApplicationControls(t *testing.T) {
	var state SidebarState
	var got SidebarResult
	attached := false
	opts := SidebarOptions{
		Conversation: "one",
		Agents:       []Option{{ID: "claude", Label: "Claude Code"}, {ID: "codex", Label: "Codex"}}, Agent: "claude",
		Models: []Option{{ID: "opus", Label: "Opus"}, {ID: "sonnet", Label: "Sonnet"}}, Model: "opus",
		RefreshAgents: true,
		Composer: ComposerOptions{
			Accessory: func(c *ui.Context) { attached = attached || IconAction(c, "Attach files", lucide.Paperclip).Clicked() },
			Footer:    text("Branch main"),
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { collect(&got, Sidebar(c, &state, opts)) }, 360, 640)
	if !tt.HasText("Branch main") {
		t.Fatal("the application's footer was replaced")
	}
	if err := tt.Click("Attach files"); err != nil || !attached {
		t.Fatalf("the application's accessory was replaced: %v", err)
	}
	for _, step := range [][2]string{{"Opus", "Sonnet"}, {"Claude Code", "Codex"}} {
		if err := tt.Click(step[0]); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click(step[1]); err != nil {
			t.Fatal(err)
		}
	}
	if got.Model != "sonnet" || got.Agent != "codex" {
		t.Fatalf("choices: model=%q agent=%q", got.Model, got.Agent)
	}
	if err := tt.Click("Refresh agents"); err != nil || !got.RefreshAgents {
		t.Fatalf("refresh: %v", err)
	}
}
