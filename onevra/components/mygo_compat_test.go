package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestGrowingControlsKeepContentHeight(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := "Name"
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Column(c).Fill().Children(func() {
				ui.Row(c).Children(func() {
					ui.Column(c).Grow(1).Children(func() {
						Button(c, "Save", ButtonSecondary).Grow(1)
						TextBox(c, &value, TextBoxOptions{}).Label("Name").Grow(1)
					})
				})
				Text(c, "After")
			})
		}, 400, 300)
		button, buttonOK := tt.Find("Save")
		input, inputOK := tt.Find("Name")
		after, afterOK := tt.Find("After")
		if !buttonOK || !inputOK || !afterOK {
			t.Fatal("missing controls")
		}
		if button.H <= 0 || input.H <= 0 || input.Y < button.Y+button.H || after.Y < input.Y+input.H {
			t.Fatalf("growing controls lost their content height: button=%+v input=%+v after=%+v", button, input, after)
		}
	}
}

func TestListHorizontalScrollWithHeader(t *testing.T) {
	for _, dark := range []bool{false, true} {
		state := ListState{}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			ui.Column(c).Fill().Children(func() {
				header := ui.Row(c).Height(24).Shrink(0).Clip().Children(func() {
					Text(c, "Header").Width(500).Shrink(0)
					Text(c, "Size").Width(100).Shrink(0)
				})
				list := List(c, &state, 50, func(i int) {
					ui.Row(c).Height(24).Children(func() {
						Text(c, "Row").Width(500).Shrink(0)
						ui.Textf(c, "%d KB", i).Width(100).Shrink(0)
					})
				}).Grow(1).RowWidth(600)
				header.ScrollWith(list)
			})
		}, 300, 200)
		header, headerOK := tt.Find("Size")
		cell, cellOK := tt.Find("0 KB")
		if !headerOK || !cellOK || header.X != 500 || cell.X != 500 {
			t.Fatalf("wide rows not preserved: header=%+v cell=%+v", header, cell)
		}
		tt.Scroll(150, 100, 300, 0)
		header, headerOK = tt.Find("Size")
		cell, cellOK = tt.Find("0 KB")
		if !headerOK || !cellOK || header.X != 200 || cell.X != 200 {
			t.Fatalf("header and rows did not scroll together: header=%+v cell=%+v", header, cell)
		}
		tt.Scroll(150, 100, 0, 120)
		after, ok := tt.Find("Size")
		if !ok || after.X != header.X || after.Y != header.Y {
			t.Fatalf("vertical scrolling moved the header: before=%+v after=%+v", header, after)
		}
	}
}
