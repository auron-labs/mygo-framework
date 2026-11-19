package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

// group wraps activities in one step for tests.
func group(title string, activities ...Activity) []Step {
	return []Step{{ID: "group", Title: title, Activities: activities, Group: &GroupState{}}}
}

func TestTurnCompletionAndUserOverride(t *testing.T) {
	var state TurnState
	builds := 0
	opts := TurnOptions{
		ID: "turn", Summary: "Worked for 1m", Working: "Working", Status: Running,
		Steps: []Step{{ID: "m", Message: func(c *ui.Context) { builds++; components.Text(c, "Tool output").Selectable() }}},
	}
	tt := ui.NewTester(func(c *ui.Context) { Turn(c, &state, opts) }, 400, 400)
	if !tt.HasText("Tool output") || !tt.HasText("Working") {
		t.Fatal("running content missing")
	}
	opts.Status = Completed
	tt.Frame()
	if !tt.HasText("Tool output") || tt.HasText("Working") {
		t.Fatal("completion without final hid the only answer")
	}
	opts.Final = func(c *ui.Context) { components.Text(c, "Final response") }
	builds = 0
	tt.Frame()
	if builds != 0 || tt.HasText("Tool output") || !tt.HasText("Final response") {
		t.Fatal("completed steps were not lazy")
	}
	if err := tt.Click("Worked for 1m"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Tool output") {
		t.Fatal("cannot reopen completion")
	}
	tt.Frame()
	if !tt.HasText("Tool output") {
		t.Fatal("stream rebuild reset manual expansion")
	}
	tt.Key(0, ui.KeySpace)
	builds = 0
	tt.Frame()
	if builds != 0 {
		t.Fatal("closing still built hidden content")
	}
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText("Tool output") {
		t.Fatal("keyboard expansion failed")
	}
}

func TestTurnProtectsReadingAndExceptionalStates(t *testing.T) {
	var folded TurnState
	if folded.Expanded(Completed, true, false) || folded.Expanded(Completed, true, true) {
		t.Fatal("scrolling through history reopened a completed turn")
	}
	for _, status := range []Status{Running, Waiting, Failed, Cancelled} {
		var s TurnState
		if !s.Expanded(status, true, false) {
			t.Fatalf("status %v folded", status)
		}
	}
	var state TurnState
	opts := TurnOptions{
		ID: "turn", Summary: "Worked", Status: Running,
		Steps: []Step{{ID: "m", Message: func(c *ui.Context) { components.Button(c, "Review", components.ButtonSecondary) }}},
		Final: func(c *ui.Context) { components.Text(c, "Answer") },
	}
	tt := ui.NewTester(func(c *ui.Context) { Turn(c, &state, opts) }, 400, 400)
	if err := tt.Click("Review"); err != nil {
		t.Fatal(err)
	}
	opts.Status = Completed
	tt.Frame()
	if !tt.HasText("Review") || !tt.Focused("Review") {
		t.Fatal("completion removed focused content")
	}
	state.SetExpanded(false)
	opts.HoldOpen = true
	tt.Frame()
	if tt.HasText("Review") {
		t.Fatal("reading policy overrode explicit collapse")
	}
	state.ResetExpansion()
	tt.Frame()
	if !tt.HasText("Review") {
		t.Fatal("hold did not defer auto collapse")
	}
}

func TestActivityIdentityAcrossOutOfOrderResults(t *testing.T) {
	var state TurnState
	draftA, draftB := "A", "B"
	sa, sb := &ActivityState{Expanded: true}, &ActivityState{Expanded: true}
	a := Activity{ID: "call-a", Action: "Run", Detail: "a", Status: Running, State: sa, Content: func(c *ui.Context) { components.TextBox(c, &draftA, components.TextBoxOptions{}).Label("Input A") }}
	b := Activity{ID: "call-b", Action: "Run", Detail: "b", Status: Running, State: sb, Content: func(c *ui.Context) { components.TextBox(c, &draftB, components.TextBoxOptions{}).Label("Input B") }}
	opts := TurnOptions{ID: "turn", Status: Running, Steps: group("Running 2 commands", a, b)}
	tt := ui.NewTester(func(c *ui.Context) { Turn(c, &state, opts) }, 400, 500)
	if err := tt.Click("Input B"); err != nil {
		t.Fatal(err)
	}
	b.Status = Completed
	opts.Steps = group("Running 2 commands", b, a)
	tt.Frame()
	if !tt.Focused("Input B") {
		t.Fatal("reordering reassigned tool focus")
	}
	tt.Type("!")
	if draftA != "A" || draftB == "B" {
		t.Fatal("update targeted wrong tool")
	}
}

func TestConversationFollowAndJump(t *testing.T) {
	var state ConversationState
	height := float32(800)
	tt := ui.NewTester(func(c *ui.Context) {
		Conversation(c, &state, "Latest", func(c *ui.Context) { ui.Box(c).Height(height).Shrink(0) })
	}, 300, 300)
	if state.Scroll.MaxY <= 0 || !state.AtLatest() {
		t.Fatalf("initial position %+v", state.Scroll)
	}
	height += 100
	tt.Frame()
	if !state.AtLatest() {
		t.Fatal("growth lost bottom")
	}
	tt.Scroll(100, 100, 0, -180)
	before := state.Scroll.Y
	if state.AtLatest() {
		t.Fatal("wheel did not release follow")
	}
	height += 100
	tt.Frame()
	if state.Scroll.Y != before {
		t.Fatalf("reading moved from %v to %v", before, state.Scroll.Y)
	}
	if err := tt.Click("Latest"); err != nil {
		t.Fatal(err)
	}
	if !state.AtLatest() {
		t.Fatal("jump did not resume follow")
	}
	height = 400
	tt.Frame()
	if !state.AtLatest() {
		t.Fatal("fold lost bottom")
	}
}

func TestConversationExpansionKeepsHeaderAndComposer(t *testing.T) {
	for _, floating := range []bool{false, true} {
		t.Run(fmt.Sprintf("floating=%v", floating), func(t *testing.T) {
			var scroll ConversationState
			var layout FloatingConversationState
			var state TurnState
			draft := ""
			tt := ui.NewTester(func(c *ui.Context) {
				content := func(c *ui.Context) {
					Turn(c, &state, TurnOptions{
						ID: "turn", Summary: "Details", Status: Completed,
						Steps: []Step{{ID: "large", Message: func(c *ui.Context) { ui.Box(c).Height(1500).Shrink(0) }}},
						Final: func(c *ui.Context) { components.Text(c, "Answer") },
					})
				}
				ui.Column(c).Fill().Children(func() {
					if floating {
						FloatingConversation(c, &layout, &scroll, "Latest", content, func(c *ui.Context) {
							GlassComposer(c, &layout, &draft, ComposerOptions{Label: "Draft", SendLabel: "Send"})
						})
					} else {
						Conversation(c, &scroll, "Latest", content)
						Composer(c, &draft, ComposerOptions{Label: "Draft", SendLabel: "Send"})
					}
				})
			}, 360, 500)
			before, _ := tt.Find("Details")
			if err := tt.Click("Details"); err != nil {
				t.Fatal(err)
			}
			after, _ := tt.Find("Details")
			if before.Y != after.Y {
				t.Fatalf("disclosure moved from %v to %v; scroll=%+v texts=%v", before.Y, after.Y, scroll.Scroll, tt.Texts())
			}
			send, ok := tt.Find("Send")
			if !ok || send.Y+send.H > 500 {
				t.Fatalf("composer pushed out of viewport: %+v", send)
			}
			if scroll.Scroll.MaxY <= 0 || scroll.AtLatest() {
				t.Fatal("expansion did not pause follow")
			}
		})
	}
}

func BenchmarkCompletedTurn(b *testing.B) {
	for _, count := range []int{10, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var state TurnState
			steps := make([]Step, count)
			for i := range steps {
				steps[i] = Step{ID: fmt.Sprint(i), Message: func(*ui.Context) { b.Fatal("hidden content built") }}
			}
			tt := ui.NewTester(func(c *ui.Context) {
				Turn(c, &state, TurnOptions{
					ID: "turn", Summary: "Completed", Status: Completed, Steps: steps,
					Final: func(c *ui.Context) { components.Text(c, "Answer") },
				})
			}, 400, 300)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tt.Frame()
			}
		})
	}
}

// A theme that leaves Inverse zero, as Onevra's palettes do, must still
// paint Send rather than leave an invisible gap.
func TestComposerSendVisibleWithoutInverse(t *testing.T) {
	for _, running := range []bool{false, true} {
		draft := "next"
		tt := ui.NewTester(func(c *ui.Context) {
			Composer(c, &draft, ComposerOptions{Label: "Message", SendLabel: "Send", StopLabel: "Stop", Running: running})
		}, 400, 160)
		tt.Frame()
		r, ok := tt.Find("Send")
		if !ok {
			t.Fatalf("running=%v: Send missing", running)
		}
		// The light theme's Text fill is dark against the light card.
		if px := tt.Image().RGBAAt(int(r.X+4), int(r.Y+r.H/2)); px.R > 128 {
			t.Fatalf("running=%v: Send fill %v is not painted", running, px)
		}
	}
}

// OnPaste is optional: pasting text without a hook inserts it normally.
func TestComposerPasteWithoutHook(t *testing.T) {
	draft := ""
	tt := ui.NewTester(func(c *ui.Context) {
		Composer(c, &draft, ComposerOptions{Label: "Message", SendLabel: "Send", StopLabel: "Stop"})
	}, 400, 160)
	_ = tt.Click("Message")
	tt.SetClipboard("pasted")
	tt.Command("paste")
	tt.Frame()
	if draft != "pasted" {
		t.Fatalf("draft = %q", draft)
	}
}

// AcceptFiles reports the paths dropped on the card; a composer without it
// takes no files.
func TestComposerAcceptsDroppedFiles(t *testing.T) {
	for _, accept := range []bool{false, true} {
		draft := ""
		var dropped []string
		tt := ui.NewTester(func(c *ui.Context) {
			if r := Composer(c, &draft, ComposerOptions{Label: "Message", SendLabel: "Send", StopLabel: "Stop", AcceptFiles: accept}); r.Dropped != nil {
				dropped = r.Dropped
			}
		}, 400, 160)
		r, ok := tt.Find("Message")
		if !ok {
			t.Fatal("composer missing")
		}
		taken := tt.DropFiles(r.X+r.W/2, r.Y+r.H/2, "/tmp/a.png", "/tmp/b.txt")
		if taken != accept || accept != (len(dropped) == 2 && dropped[0] == "/tmp/a.png" && dropped[1] == "/tmp/b.txt") {
			t.Fatalf("accept=%v: taken %v, dropped %q", accept, taken, dropped)
		}
		if draft != "" {
			t.Fatalf("accept=%v: drop changed the draft to %q", accept, draft)
		}
	}
}

func TestComposerSendDuringComposition(t *testing.T) {
	for _, glass := range []bool{false, true} {
		for _, running := range []bool{false, true} {
			for _, disabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("glass=%v/running=%v/disabled=%v", glass, running, disabled), func(t *testing.T) {
					draft := "ready"
					sends := 0
					var composing bool
					tt := ui.NewTester(func(c *ui.Context) {
						options := ComposerOptions{Label: "Message", SendLabel: "Send", StopLabel: "Stop", Running: running, SendDisabled: disabled}
						var result ComposerResult
						if glass {
							result = GlassComposer(c, nil, &draft, options)
						} else {
							result = Composer(c, &draft, options)
						}
						composing = result.Input.Composing()
						if result.Send {
							sends++
						}
					}, 400, 240)
					if err := tt.Click("Message"); err != nil {
						t.Fatal(err)
					}
					tt.Compose("ni", 2)
					if !composing {
						t.Fatal("input is not composing")
					}
					tt.Key(0, ui.KeyEnter)
					if sends != 0 {
						t.Fatal("candidate-selection Enter sent the draft")
					}
					if _, ok := tt.Find("Stop"); ok {
						t.Fatal("stop shown beside send")
					}
					if err := tt.Click("Send"); err != nil {
						t.Fatal(err)
					}
					want := 1
					if disabled {
						want = 0
					}
					if sends != want {
						t.Fatalf("sends = %d, want %d", sends, want)
					}
				})
			}
		}
	}
}

func TestComposerSubmitOnEnterAndSteering(t *testing.T) {
	for _, glass := range []bool{false, true} {
		t.Run(fmt.Sprintf("glass=%v", glass), func(t *testing.T) {
			composer := Composer
			if glass {
				composer = func(c *ui.Context, draft *string, options ComposerOptions) ComposerResult {
					return GlassComposer(c, nil, draft, options)
				}
			}
			draft := ""
			sends, stops := 0, 0
			running, stopDisabled := false, false
			tt := ui.NewTester(func(c *ui.Context) {
				p := composer(c, &draft, ComposerOptions{
					Label: "Message", SendLabel: "Send", StopLabel: "Stop",
					Running: running, StopDisabled: stopDisabled,
				})
				if p.Send {
					sends++
				}
				if p.Stop {
					stops++
				}
			}, 400, 240)
			_ = tt.Click("Message")
			tt.Key(0, ui.KeyEnter)
			if sends != 0 {
				t.Fatal("empty draft sent")
			}
			tt.Compose("ni", 2)
			tt.Key(0, ui.KeyEnter)
			if sends != 0 {
				t.Fatal("composition sent")
			}
			tt.Type("你")
			lines := strings.Count(draft, "\n")
			tt.Key(ui.Shift, ui.KeyEnter)
			tt.Type("b")
			if sends != 0 || strings.Count(draft, "\n") != lines+1 {
				t.Fatalf("shift+enter: sends=%d draft=%q", sends, draft)
			}
			before := draft
			tt.Key(0, ui.KeyEnter)
			if sends != 1 || draft != before {
				t.Fatalf("enter: sends=%d draft=%q", sends, draft)
			}
			running = true
			tt.Frame()
			if _, ok := tt.Find("Stop"); ok {
				t.Fatal("stop shown beside send for a nonempty draft")
			}
			if err := tt.Click("Send"); err != nil || sends != 2 {
				t.Fatalf("steer send unavailable: %v %d", err, sends)
			}
			draft = " \n\t"
			tt.Frame()
			if _, ok := tt.Find("Send"); ok {
				t.Fatal("send shown for an empty draft while running")
			}
			if err := tt.Click("Stop"); err != nil || stops != 1 {
				t.Fatal("stop failed")
			}
			stopDisabled = true
			tt.Frame()
			if err := tt.Click("Stop"); err != nil || stops != 1 {
				t.Fatalf("disabled stop: %v %d", err, stops)
			}
		})
	}
}
