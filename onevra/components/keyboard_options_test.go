package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// MyGo's headless Tester does not read an editor's keyboard traits back, so
// the pass-through tests use the one trait it does act on: ReturnNext moves
// the focus after a single-line input submits.
func TestTextBoxKeyboardOptionsReachEditor(t *testing.T) {
	for _, tc := range []struct {
		returnKey ui.ReturnKeyType
		moves     bool
	}{
		{ui.ReturnDefault, false},
		{ui.ReturnNext, true},
	} {
		email, subject := "", ""
		tt := ui.NewTester(func(c *ui.Context) {
			TextBox(c, &email, TextBoxOptions{ReturnKey: tc.returnKey}).Label("Email")
			TextBox(c, &subject, TextBoxOptions{}).Label("Subject")
		}, 300, 120)
		if err := tt.Click("Email"); err != nil {
			t.Fatal(err)
		}
		tt.Type("hi")
		tt.Key(0, ui.KeyEnter)
		if moved := tt.Focused("Subject"); moved != tc.moves {
			t.Fatalf("return key %v: focus moved=%v, want %v", tc.returnKey, moved, tc.moves)
		}
		if email != "hi" {
			t.Fatalf("value = %q", email)
		}
	}
}

func TestNumberInputKeyboardOptionsReachEditor(t *testing.T) {
	amount, subject := 0.0, ""
	tt := ui.NewTester(func(c *ui.Context) {
		NumberInput(c, &amount, 0, 10, 0.5, NumberInputOptions{
			HideStepper: true,
			ReturnKey:   ui.ReturnNext,
		}).Label("Amount")
		TextBox(c, &subject, TextBoxOptions{}).Label("Subject")
	}, 300, 120)
	if err := tt.Click("Amount"); err != nil {
		t.Fatal(err)
	}
	tt.Type("3")
	tt.Key(0, ui.KeyEnter)
	if !tt.Focused("Subject") {
		t.Fatal("number input kept the focus after Enter")
	}
	if amount != 3 {
		t.Fatalf("value = %v, want 3", amount)
	}
}

func TestNumberKeyboardDefaults(t *testing.T) {
	for _, tc := range []struct {
		lo, hi, step float64
		want         ui.KeyboardType
	}{
		{0, 100, 1, ui.KeyboardNumber},
		{-10, 10, 1, ui.KeyboardNumber},
		{0, 1, 0.1, ui.KeyboardDecimal},
		{0, 10, 0.25, ui.KeyboardDecimal},
		{0.5, 10, 1, ui.KeyboardDecimal},
	} {
		if got := numberKeyboard(ui.KeyboardDefault, tc.lo, tc.hi, tc.step); got != tc.want {
			t.Errorf("numberKeyboard(%v, %v, %v) = %v, want %v", tc.lo, tc.hi, tc.step, got, tc.want)
		}
	}
	if got := numberKeyboard(ui.KeyboardEmail, 0, 1, 0.1); got != ui.KeyboardEmail {
		t.Errorf("explicit keyboard overridden: %v", got)
	}
}
