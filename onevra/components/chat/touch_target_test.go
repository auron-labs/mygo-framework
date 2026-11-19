package chat

import (
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// coarse are the desktop settings a touch screen alone reports: the chat's
// compact controls grow to components.TouchMin under them, where the other
// tests hold them to their compact sizes.
func coarse() ui.Preferences {
	return ui.Preferences{
		TextScale: 1, Pointer: ui.PointerCoarse, AnyPointer: ui.PointerCoarse,
		NoHover: true, NoAnyHover: true,
	}
}

func TestChatControlsTouchTarget(t *testing.T) {
	removed := 0
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(10).Gap(10).Children(func() {
			Chip(c, "Model", ChipOptions{Caret: true})
			IconAction(c, "Attach", lucide.Paperclip)
			Attachment(c, AttachmentOptions{Label: "Notes.txt", Detail: "2 KB", RemoveLabel: "Remove file"})
			if AttachmentPreview(c, AttachmentPreviewOptions{Label: "Photo", RemoveLabel: "Remove image"}).Remove {
				removed++
			}
			Quota(c, QuotaOptions{ID: "acct", Title: "Usage", RefreshLabel: "Refresh usage", Rows: []QuotaRow{
				{Label: "Messages", Value: "12 / 40", Fraction: .3},
			}})
		})
	}, 400, 500, ui.TesterOptions{Preferences: coarse()})
	for _, label := range []string{"Model", "Attach", "Remove file", "Remove image", "Refresh usage"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q", label)
		}
		if r.W < 44 || r.H < 44 {
			t.Fatalf("%q hit target %+v, want at least a 44×44 square", label, r)
		}
	}
	// A press on the preview's grown square removes the attachment, while the
	// badge it holds stays a badge.
	if err := tt.Click("Remove image"); err != nil || removed != 1 {
		t.Fatalf("removal: %v count=%d", err, removed)
	}
}

func TestContextMeterTouchTarget(t *testing.T) {
	var state MeterState
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Row(c).Padding(10).Children(func() {
			ContextMeter(c, &state, ContextMeterOptions{Label: "Usage meter", Title: "Usage", Value: "3 of 5", Fraction: .6})
		})
	}, 400, 200, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Usage meter")
	if !ok {
		t.Fatal("missing meter")
	}
	if r.W < 44 || r.H < 44 {
		t.Fatalf("meter hit target %+v, want at least a 44×44 square", r)
	}
}

// TestActivityGroupTouchTarget checks that a turn's activity disclosure row
// takes the coarse pointer's minimum height.
func TestActivityGroupTouchTarget(t *testing.T) {
	var turn TurnState
	g := &GroupState{}
	opts := TurnOptions{ID: "turn", Status: Running, Steps: []Step{{ID: "g", Title: "Ran 2 tools", Group: g, Activities: []Activity{
		{ID: "a", Action: "Read", Detail: "a.go"},
	}}}}
	tt := ui.NewTesterWith(func(c *ui.Context) {
		Turn(c, &turn, opts)
	}, 400, 400, ui.TesterOptions{Preferences: coarse()})
	r, ok := tt.Find("Ran 2 tools")
	if !ok {
		t.Fatal("missing disclosure row")
	}
	if r.H < 44 {
		t.Fatalf("disclosure row %+v, want at least 44 high", r)
	}
}

// TestFloatingJumpTouchTarget checks that the jump-to-latest control takes
// the coarse pointer's minimum square above the composer.
func TestFloatingJumpTouchTarget(t *testing.T) {
	var layout FloatingConversationState
	var state ConversationState
	draft := "Reply"
	tt := ui.NewTesterWith(func(c *ui.Context) {
		FloatingConversation(c, &layout, &state, "Latest", func(c *ui.Context) {
			for i := range 20 {
				components.Text(c, fmt.Sprintf("Message %d", i)).Height(40).Shrink(0)
			}
		}, func(c *ui.Context) {
			GlassComposer(c, &layout, &draft, ComposerOptions{Label: "Reply", SendLabel: "Send"})
		})
	}, 900, 600, ui.TesterOptions{Preferences: coarse()})
	state.Scroll.Y = 0
	tt.Frame()
	r, ok := tt.Find("Latest")
	if !ok {
		t.Fatal("missing jump control")
	}
	if r.W < 44 || r.H < 44 {
		t.Fatalf("jump hit target %+v, want at least a 44×44 square", r)
	}
}
