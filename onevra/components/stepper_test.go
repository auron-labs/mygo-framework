package components

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

var stepperSteps = []StepperStep{
	{Label: "Account", Description: "Your email"},
	{Label: "Shipping", Description: "Where it goes"},
	{Label: "Payment"},
	{Label: "Review"},
}

func TestStepperStatusLabels(t *testing.T) {
	for _, tc := range []struct {
		current int
		want    []string
	}{
		{-3, []string{"Account, Current step", "Shipping, Not started"}},
		{2, []string{"Account, Completed", "Shipping, Completed", "Payment, Current step", "Review, Not started"}},
		{9, []string{"Account, Completed", "Review, Completed"}},
	} {
		tt := ui.NewTester(func(c *ui.Context) {
			Stepper(c, stepperSteps, tc.current, StepperOptions{})
		}, 600, 160)
		for _, label := range tc.want {
			if _, ok := tt.Find(label); !ok {
				t.Errorf("current %d: missing %q", tc.current, label)
			}
		}
		if _, ok := tt.Find("Progress"); !ok {
			t.Errorf("current %d: missing default name", tc.current)
		}
	}
}

func TestStepperDescriptionsAndErrors(t *testing.T) {
	steps := slices.Clone(stepperSteps)
	steps[0].Error = "Email is taken"
	tt := ui.NewTester(func(c *ui.Context) {
		Stepper(c, steps, 1, StepperOptions{CurrentDescription: true})
	}, 600, 160)
	if !tt.HasText("Email is taken") || tt.HasText("Your email") {
		t.Fatal("an error replaces its description")
	}
	if !tt.HasText("Where it goes") {
		t.Fatal("missing the current description")
	}
	if _, ok := tt.Find("Account, Error"); !ok {
		t.Fatal("missing error status")
	}
}

func TestStepperSelection(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		chosen := -1
		tt := ui.NewTester(func(c *ui.Context) {
			if i, ok := Stepper(c, stepperSteps, 2, StepperOptions{Selectable: selectable}).Chosen(); ok {
				chosen = i
			}
		}, 600, 160)
		for _, label := range []string{"Payment, Current step", "Review, Not started"} {
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
		}
		if chosen != -1 {
			t.Fatalf("selectable %v: chose %d beyond the completed steps", selectable, chosen)
		}
		if err := tt.Click("Shipping, Completed"); err != nil {
			t.Fatal(err)
		}
		if want := map[bool]int{false: -1, true: 1}[selectable]; chosen != want {
			t.Fatalf("selectable %v: chose %d, want %d", selectable, chosen, want)
		}
	}
}

func TestStepperKeyboard(t *testing.T) {
	chosen := -1
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			Button(c, "Before", ButtonSecondary)
			if i, ok := Stepper(c, stepperSteps, 2, StepperOptions{Selectable: true}).Chosen(); ok {
				chosen = i
			}
		})
	}, 600, 200)
	if err := tt.Click("Before"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Account, Completed") {
		t.Fatal("Tab did not enter the first step")
	}
	tt.Key(0, ui.KeyEnd)
	if !tt.Focused("Payment, Current step") {
		t.Fatal("End did not reach the current step")
	}
	tt.Key(0, ui.KeyLeft)
	if !tt.Focused("Shipping, Completed") {
		t.Fatal("Left did not move back")
	}
	tt.Key(0, ui.KeyEnter)
	if chosen != 1 {
		t.Fatalf("Enter chose %d", chosen)
	}
}

func TestStepperAnnouncesProgress(t *testing.T) {
	current := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Stepper(c, stepperSteps, current, StepperOptions{CompleteLabel: "Done"})
	}, 600, 160)
	if len(tt.Announcements()) != 0 {
		t.Fatal("announced the first frame")
	}
	current = 2
	tt.Frame()
	current = len(stepperSteps)
	tt.Frame()
	got := tt.Announcements()
	if want := []string{"Step 3 of 4: Payment", "Done"}; !slices.Equal(got, want) {
		t.Fatalf("announcements %q, want %q", got, want)
	}
}

func TestStepperRendering(t *testing.T) {
	steps := slices.Clone(stepperSteps)
	steps[3].Error = "Card declined"
	for _, vertical := range []bool{false, true} {
		for _, compact := range []bool{false, true} {
			for _, dark := range []bool{false, true} {
				t.Run(fmt.Sprint(vertical, "/", compact, "/", dark), func(t *testing.T) {
					current := 1
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						Stepper(c, steps, current, StepperOptions{Orientation: map[bool]Orientation{false: Horizontal, true: Vertical}[vertical], Compact: compact, Selectable: true, Label: "Checkout"})
					}, 360, 360)
					if r, ok := tt.Find("Checkout"); !ok || r.W <= 0 || r.H <= 0 {
						t.Fatal("missing stepper bounds")
					}
					if tt.Image() == nil {
						t.Fatal("missing rendering")
					}
					// A jump across steps staggers, then settles.
					current = 3
					tt.Frame()
					tt.Advance(time.Second)
					if tt.Image() == nil || tt.FramePending() {
						t.Fatal("motion did not settle")
					}
					if compact && !vertical && !tt.HasText("Card declined") {
						t.Fatal("missing the compact caption")
					}
				})
			}
		}
	}
}
