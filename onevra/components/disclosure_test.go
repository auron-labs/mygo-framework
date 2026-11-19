package components

import (
	"bytes"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestDisclosureAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, accordion := range []bool{false, true} {
			for _, open := range []bool{false, true} {
				build := func(upstream bool) *ui.Tester {
					return ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						body := func() { Text(c, "Content") }
						if accordion {
							if upstream {
								ui.Accordion(c, func() { ui.AccordionItem(c, "Details", &open, body); ui.AccordionItem(c, "Other", &open, body) })
							} else {
								Accordion(c, func(g *AccordionGroup) { g.Item("Details", &open, body); g.Item("Other", &open, body) })
							}
						} else if upstream {
							ui.Collapsible(c, "Details", &open, body)
						} else {
							Collapsible(c, "Details", &open, CollapsibleOptions{ArrowPosition: ArrowLeft}, body)
						}
					}, 320, 240)
				}
				ours, upstream := build(false), build(true)
				before := bytes.Clone(ours.Image().Pix)
				if !bytes.Equal(before, upstream.Image().Pix) {
					t.Fatalf("resting appearance differs: dark=%v accordion=%v open=%v", dark, accordion, open)
				}
				r, _ := ours.Find("Details")
				ours.Move(r.X+r.W/2, r.Y+r.H/2)
				if !bytes.Equal(before, ours.Image().Pix) {
					t.Fatal("hover changed appearance")
				}
			}
		}
	}
}

func TestCollapsibleInteractionAndPlacement(t *testing.T) {
	for _, position := range []ArrowPosition{ArrowRight, ArrowLeft} {
		open, disabled, changes := false, false, 0
		var trigger ui.Rect
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			ui.Column(c).Disabled(disabled).Children(func() {
				p := Collapsible(c, "Details", &open, CollapsibleOptions{ArrowPosition: position}, func() { Button(c, "Inside", ButtonSecondary) })
				p.Trigger.DrawOver(func(_ *ui.Painter, r ui.Rect) { trigger = r })
				if p.Changed() {
					changes++
				}
			})
		}, 320, 240)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		label, _ := tt.Find("Details")
		left, right := label.X-trigger.X, trigger.X+trigger.W-label.X-label.W
		if position == ArrowRight && right <= left || position == ArrowLeft && left <= right {
			t.Fatalf("wrong arrow position: left=%v right=%v", left, right)
		}
		if tt.HasText("Inside") {
			t.Fatal("closed content built")
		}
		if err := tt.Click("Details"); err != nil {
			t.Fatal(err)
		}
		if !open || changes != 1 || !tt.HasText("Inside") {
			t.Fatalf("click: open=%v changes=%d", open, changes)
		}
		tt.Key(0, ui.KeyTab)
		if !tt.Focused("Inside") {
			t.Fatal("Tab skipped panel")
		}
		tt.Key(ui.Shift, ui.KeyTab)
		tt.Key(0, ui.KeySpace)
		if open || changes != 2 || tt.HasText("Inside") {
			t.Fatal("Space did not close")
		}
		tt.Key(0, ui.KeyEnter)
		if !open || changes != 3 {
			t.Fatal("Enter did not open")
		}
		disabled = true
		tt.Frame()
		_ = tt.Click("Details")
		tt.Key(0, ui.KeySpace)
		if !open || changes != 3 {
			t.Fatal("disabled disclosure changed")
		}
	}
}

func TestAccordionKeyboard(t *testing.T) {
	states := [3]bool{}
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Accordion(c, func(g *AccordionGroup) {
			for i, title := range []string{"General", "Privacy", "Advanced"} {
				if g.Item(title, &states[i], func() { Button(c, "Inside "+title, ButtonSecondary) }).Changed() {
					changes++
				}
			}
		})
	}, 320, 400)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Key(0, ui.KeyTab)
	for _, step := range []struct {
		key  ui.Key
		want string
	}{
		{ui.KeyDown, "Privacy"},
		{ui.KeyDown, "Advanced"},
		{ui.KeyDown, "Advanced"},
		{ui.KeyHome, "General"},
		{ui.KeyUp, "General"},
		{ui.KeyEnd, "Advanced"},
		{ui.KeyUp, "Privacy"},
	} {
		tt.Key(0, step.key)
		if !tt.Focused(step.want) {
			t.Fatalf("key %v: not focused %s", step.key, step.want)
		}
	}
	tt.Key(0, ui.KeyEnter)
	if !states[1] || changes != 1 {
		t.Fatal("Enter did not open")
	}
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Inside Privacy") {
		t.Fatal("Tab skipped panel")
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeySpace)
	if !states[0] || !states[1] || changes != 2 {
		t.Fatal("sections not independent")
	}
}

func TestCollapsibleTrailing(t *testing.T) {
	open := false
	var root, trigger ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		p := Collapsible(c, "Details", &open, CollapsibleOptions{Trailing: func() { Text(c, "42") }}, func() { Text(c, "Inside") })
		p.Root.FillWidth().Draw(func(_ *ui.Painter, r ui.Rect) { root = r })
		p.Trigger.DrawOver(func(_ *ui.Painter, r ui.Rect) { trigger = r })
	}, 320, 240)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	count, ok := tt.Find("42")
	if !ok {
		t.Fatal("trailing content missing")
	}
	if right := root.X + root.W - count.X - count.W; right > 1 || count.X < trigger.X+trigger.W {
		t.Fatalf("trailing content not at the header's end: count=%v trigger=%v root=%v", count, trigger, root)
	}
	if mid, triggerMid := count.Y+count.H/2, trigger.Y+trigger.H/2; mid < triggerMid-1 || mid > triggerMid+1 {
		t.Fatalf("trailing content not centered on the trigger: %v vs %v", mid, triggerMid)
	}
	if err := tt.Click("42"); err != nil || open {
		t.Fatalf("trailing content toggled the disclosure: %v", err)
	}
	if err := tt.Click("Details"); err != nil || !open || !tt.HasText("Inside") {
		t.Fatalf("trigger no longer toggles: %v", err)
	}
}

func TestCollapsibleLeading(t *testing.T) {
	open := false
	var icon, trigger ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		p := Collapsible(c, "Details", &open, CollapsibleOptions{Leading: func() {
			ui.Box(c).Size(12, 12).Draw(func(_ *ui.Painter, r ui.Rect) { icon = r })
		}}, func() { Text(c, "Inside") })
		p.Trigger.DrawOver(func(_ *ui.Painter, r ui.Rect) { trigger = r })
	}, 320, 240)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	label, ok := tt.Find("Details")
	if !ok {
		t.Fatal("label missing")
	}
	if icon.X < trigger.X || icon.X+icon.W > label.X {
		t.Fatalf("leading content not inside the trigger before the label: icon=%v label=%v trigger=%v", icon, label, trigger)
	}
	tt.ClickAt(icon.X+icon.W/2, icon.Y+icon.H/2)
	if !open {
		t.Fatal("leading content does not toggle")
	}
}

func TestDisclosureAnimation(t *testing.T) {
	open := false
	var root ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		p := Collapsible(c, "Details", &open, CollapsibleOptions{}, func() { ui.Box(c).Height(100) })
		p.Root.Draw(func(_ *ui.Painter, r ui.Rect) { root = r })
	}, 320, 240)
	closed := root.H
	_ = tt.Click("Details")
	tt.Advance(60 * time.Millisecond)
	tt.Frame()
	middle := root.H
	tt.Advance(220 * time.Millisecond)
	tt.Frame()
	if middle <= closed || middle >= root.H {
		t.Fatalf("no expansion animation: %v %v %v", closed, middle, root.H)
	}
	_ = tt.Click("Details")
	tt.Advance(60 * time.Millisecond)
	tt.Frame()
	if root.H <= closed {
		t.Fatal("closing snapped shut")
	}
	tt.Advance(220 * time.Millisecond)
	tt.Frame()
	if root.H != closed {
		t.Fatal("closing did not finish")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	_ = tt.Click("Details")
	if root.H <= closed {
		t.Fatal("reduced motion did not open immediately")
	}
	_ = tt.Click("Details")
	if root.H != closed {
		t.Fatal("reduced motion animated closing")
	}
}
