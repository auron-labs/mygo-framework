package components

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestComponentsDoNotSetFrameTheme(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range files {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "SetTheme" {
				t.Errorf("%s: components must not set the frame theme", name)
			}
			return true
		})
	}
}

func TestCollectionsPreserveRootBackground(t *testing.T) {
	for _, name := range []string{"List", "Tree", "TreeVirtualized"} {
		for _, dark := range []bool{false, true} {
			for _, background := range []ui.Color{ui.Transparent, ui.Hex("#473b8c")} {
				t.Run(fmt.Sprintf("%s/%v/%v", name, dark, background), func(t *testing.T) {
					selected, open := 0, true
					list := ListState{Selected: &selected}
					var tree TreeVirtualizedState[int]
					tree.Open.Add(-1)
					items := make([]int, 10000)
					for i := range items {
						items[i] = i
					}
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						original := c.Theme()
						c.Root().Background(background)
						switch name {
						case "List":
							List(c, &list, len(items), func(i int) {
								if c.Theme() != original {
									t.Error("row changed the theme")
								}
								ui.Textf(c, "Item %d", i).Padding(8)
							}).Size(200, 160)
						case "Tree":
							Tree(c, func() {
								TreeItem(c, "Folder", &open, true, func() {
									TreeItem(c, "Child", nil, false, nil)
								})
							}).Width(200)
						case "TreeVirtualized":
							TreeVirtualized(c, &tree, []int{-1}, func(i int) []int {
								if i == -1 {
									return items
								}
								return nil
							}, func(i int) { ui.Textf(c, "Item %d", i) }).Size(200, 160)
						}
						if c.Theme() != original {
							t.Error("component changed the theme")
						}
					}, 260, 240)
					assertBackground := func() {
						t.Helper()
						got := tt.Image().RGBAAt(250, 230)
						if got.A != background.A {
							t.Fatalf("root alpha = %d, want %d", got.A, background.A)
						}
						if background.A == 255 && (got.R != background.R || got.G != background.G || got.B != background.B) {
							t.Fatalf("root background = %v, want %v", got, background)
						}
					}
					assertBackground()
					switch name {
					case "List":
						list.ScrollToEnd()
					case "TreeVirtualized":
						tree.List.ScrollToEnd()
					case "Tree":
						open = false
					}
					tt.Frame()
					assertBackground()
					if name != "Tree" && !tt.HasText("Item 9999") {
						t.Fatal("scroll did not build late rows at the end")
					}
				})
			}
		}
	}
}
