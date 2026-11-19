package shaders

import (
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

// The background watches clicks without taking them: content inside still
// gets its click, and the background records a ripple there.
func TestRippleWithoutTakingClicks(t *testing.T) {
	var s *state
	clicks := 0
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		bg := SignalField(c, Options{})
		s = ui.Local(bg, stateKey{}, func() state { return state{} })
		bg.Children(func() {
			ui.Column(c).Padding(40).Children(func() {
				if components.Button(c, "Start", components.ButtonPrimary).Clicked() {
					clicks++
				}
			})
		})
	}, 400, 300)
	if err := tt.Click("Start"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("content got %d clicks", clicks)
	}
	tt.ClickAt(300, 250)
	if live := s.liveRipples(tt.Now()); live != 2 {
		t.Fatalf("%d live ripples, want 2", live)
	}
	if r := s.ripples[1]; r.x != 300 || r.y != 250 {
		t.Fatalf("ripple at %v, %v", r.x, r.y)
	}
	tt.Advance(rippleLife)
	if live := s.liveRipples(tt.Now()); live != 0 {
		t.Fatalf("%d ripples outlived their life", live)
	}
}

// Content taking its own input, such as a Honeycomb, hides its presses from
// the background, and reports them with Ripple.
func TestRippleForContentTakingItsInput(t *testing.T) {
	var s *state
	selected := "a"
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		bg := SignalField(c, Options{})
		s = ui.Local(bg, stateKey{}, func() state { return state{} })
		bg.Children(func() {
			items := []components.HoneycombItem{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
			if id := components.Honeycomb(c, components.HoneycombOptions{
				Items: items, Selected: selected,
				OnPress: func() { Ripple(c, bg) },
			}); id != "" {
				selected = id
			}
		})
	}, 400, 300)
	tt.Frame()
	r, ok := tt.Find("B")
	if !ok {
		t.Fatal("no item B")
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Move(x, y)
	if err := tt.Click("B"); err != nil {
		t.Fatal(err)
	}
	if selected != "b" {
		t.Fatalf("honeycomb chose %q", selected)
	}
	if live := s.liveRipples(tt.Now()); live != 1 {
		t.Fatalf("%d live ripples, want 1", live)
	}
	if r := s.ripples[0]; r.x != x || r.y != y {
		t.Fatalf("ripple at %v, %v, want %v, %v", r.x, r.y, x, y)
	}
}

func TestRipplesReplaceTheOldest(t *testing.T) {
	var s state
	now := time.Unix(100, 0)
	for i := range MaxRipples + 2 {
		s.press(float32(i), 0, now)
	}
	if live := s.liveRipples(now); live != MaxRipples {
		t.Fatalf("%d live ripples", live)
	}
	if s.ripples[0].x != MaxRipples || s.ripples[1].x != MaxRipples+1 || s.ripples[2].x != 2 {
		t.Fatal("new ripples did not replace the oldest")
	}
}

func TestInertAndReducedMotionSkipRipples(t *testing.T) {
	for _, tc := range []struct {
		name   string
		o      Options
		reduce bool
	}{{"inert", Options{Inert: true}, false}, {"reduce motion", Options{}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			var s *state
			tt := ui.NewTester(func(c *ui.Context) {
				bg := SignalField(c, tc.o)
				s = ui.Local(bg, stateKey{}, func() state { return state{} })
			}, 200, 200)
			tt.SetPreferences(ui.Preferences{ReduceMotion: tc.reduce})
			tt.ClickAt(100, 100)
			if live := s.liveRipples(tt.Now()); live != 0 {
				t.Fatalf("%d ripples", live)
			}
		})
	}
}
