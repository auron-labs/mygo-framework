package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestListKeyedSelectionAndActivation(t *testing.T) {
	items := []string{"Alpha", "Beta", "Charlie", "Delta"}
	selected, changes, submits := -1, 0, 0
	var selection ui.Selection[any]
	state := ListState{
		Selected: &selected, Selection: &selection,
		Key: func(i int) any { return items[i] }, Label: func(i int) string { return items[i] },
		OnChange: func() { changes++ }, OnSubmit: func(int) { submits++ },
	}
	tt := ui.NewTester(func(c *ui.Context) {
		List(c, &state, len(items), func(i int) { Text(c, items[i]).Padding(8) }).Size(200, 160)
	}, 240, 200)
	click := func(mods ui.Modifiers, label string) {
		t.Helper()
		if err := tt.ClickWith(mods, label); err != nil {
			t.Fatal(err)
		}
	}
	click(0, "Alpha")
	click(selectionModifier(), "Charlie")
	if selection.Len() != 2 || !selection.Has("Alpha") || !selection.Has("Charlie") || selected != 2 {
		t.Fatal("additive keyed selection failed")
	}
	click(ui.Shift, "Delta")
	if selection.Len() != 2 || !selection.Has("Charlie") || !selection.Has("Delta") {
		t.Fatal("range selection failed")
	}
	// Reordering keeps both the cursor and the range anchor with their keys.
	items = []string{"Delta", "Alpha", "Charlie", "Beta"}
	tt.Frame()
	if selected != 0 || !selection.Has("Delta") {
		t.Fatal("selection did not follow reordered keys")
	}
	click(ui.Shift, "Beta")
	if selection.Len() != 2 || !selection.Has("Charlie") || !selection.Has("Beta") {
		t.Fatal("range anchor did not follow its key")
	}
	tt.Key(selectionModifier(), ui.KeyA)
	if selection.Len() != 4 {
		t.Fatal("select all failed")
	}
	before := changes
	tt.Key(selectionModifier(), ui.KeyA)
	if changes != before {
		t.Fatal("unchanged selection emitted OnChange")
	}
	tt.Key(0, ui.KeyEnter)
	if submits != 1 {
		t.Fatalf("Enter submitted %d times", submits)
	}
	click(0, "Alpha")
	click(0, "Alpha")
	if submits != 2 {
		t.Fatalf("double click submitted %d times", submits)
	}
	// Type-to-select still operates on the focused collection, not row widgets.
	tt.Type("ch")
	if items[selected] != "Charlie" {
		t.Fatalf("type-to-select chose %d", selected)
	}
	items = []string{"Delta", "Alpha", "Beta"}
	tt.Frame()
	if selected != -1 {
		t.Fatal("removed cursor key selected an unrelated row")
	}
	items = nil
	tt.Frame()
	tt.Key(0, ui.KeyEnd)
	if selected != -1 {
		t.Fatal("empty list changed selection")
	}
}

func TestIndependentAndNestedCollections(t *testing.T) {
	a, b := 0, 0
	first, second := ListState{Selected: &a}, ListState{Selected: &b}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			List(c, &first, 3, func(i int) { ui.Textf(c, "A%d", i).Padding(8) }).Size(160, 150)
			List(c, &second, 3, func(i int) { ui.Textf(c, "B%d", i).Padding(8) }).Size(160, 150)
		})
	}, 340, 180)
	if err := tt.Click("B1"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyDown)
	if a != 0 || b != 2 {
		t.Fatal("lists share keyboard or selection state")
	}
	var nested ListState
	tt = ui.NewTester(func(c *ui.Context) {
		List(c, nil, 1, func(int) {
			List(c, &nested, 1000, func(i int) { ui.Textf(c, "Nested %d", i).Padding(8) }).Height(100).FillWidth()
		}).Size(200, 140)
	}, 220, 180)
	nested.ScrollToEnd()
	tt.Frame()
	if !tt.HasText("Nested 999") {
		t.Fatal("nested virtual list did not scroll independently")
	}
}

func TestTreeVirtualizedCollapseSelectedDescendant(t *testing.T) {
	selected := 2
	var state TreeVirtualizedState[string]
	state.List.Selected = &selected
	state.Open.Add("Root")
	state.Open.Add("Branch")
	tt := ui.NewTester(func(c *ui.Context) {
		TreeVirtualized(c, &state, []string{"Root", "Other"}, func(item string) []string {
			switch item {
			case "Root":
				return []string{"Branch"}
			case "Branch":
				return []string{"Child"}
			}
			return nil
		}, func(item string) { Text(c, item) }).Size(240, 180)
	}, 260, 200)
	r, ok := tt.Find("Root")
	if !ok {
		t.Fatal("missing root")
	}
	tt.ClickAt(12, r.Y+r.H/2)
	if state.Open.Has("Root") || selected != 0 || state.Rows() != 2 {
		t.Fatalf("collapse lost cursor: selected=%d rows=%d", selected, state.Rows())
	}
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyRight)
	if selected != 1 || state.Item(selected) != "Branch" {
		t.Fatal("Right did not move into child")
	}
	tt.Key(0, ui.KeyLeft)
	if state.Open.Has("Branch") {
		t.Fatal("Left did not collapse branch")
	}
	tt.Key(0, ui.KeyLeft)
	if selected != 0 {
		t.Fatal("Left did not select parent")
	}
}

func TestTreeFocusNavigationAndNesting(t *testing.T) {
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		Tree(c, func() {
			TreeItem(c, "Root", &open, false, func() {
				TreeItem(c, "Child", nil, false, nil)
			})
			TreeItem(c, "Sibling", nil, false, nil)
		}).Width(200)
		Tree(c, func() { TreeItem(c, "Separate", nil, false, nil) }).Width(200)
	}, 240, 240)
	if err := tt.Click("Root"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key   ui.Key
		label string
	}{
		{ui.KeyRight, "Child"},
		{ui.KeyDown, "Sibling"},
		{ui.KeyHome, "Root"},
		{ui.KeyEnd, "Sibling"},
		{ui.KeyUp, "Child"},
		{ui.KeyLeft, "Root"},
	} {
		tt.Key(0, step.key)
		if !tt.Focused(step.label) {
			t.Fatalf("key %v did not focus %s", step.key, step.label)
		}
	}
	tt.Key(0, ui.KeyLeft)
	if open || tt.HasText("Child") {
		t.Fatal("keyboard did not collapse")
	}
	// Several tree builders in one context must not share their build scope.
	for i := range 3 {
		tt.Key(0, ui.KeyDown)
		if !tt.Focused("Sibling") {
			t.Fatalf("tree navigation escaped its scope on step %d", i)
		}
	}
}
