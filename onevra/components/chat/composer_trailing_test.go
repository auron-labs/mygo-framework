package chat

import (
	"fmt"
	"math"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/egoist/mygo/ui"
)

func TestComposerTrailingLayoutAndInteraction(t *testing.T) {
	for _, glass := range []bool{false, true} {
		for _, width := range []int{320, 720} {
			for _, running := range []bool{false, true} {
				for _, draft := range []string{"", "Next message"} {
					t.Run(fmt.Sprintf("glass=%v/width=%d/running=%v/draft=%q", glass, width, running, draft), func(t *testing.T) {
						clicks, sends, stops := 0, 0, 0
						tt := ui.NewTester(func(c *ui.Context) {
							options := ComposerOptions{
								Label: "Message", SendLabel: "Send", StopLabel: "Stop", Running: running,
								Accessory: func(c *ui.Context) {
									components.Text(c, "A long model label with reasoning and service tier").MinWidth(0).SingleLine().Ellipsis("…")
								},
								Trailing: func(c *ui.Context) {
									if ui.ButtonBase(c).Label("Details").Size(24, 20).Clicked() {
										clicks++
									}
								},
							}
							var result ComposerResult
							if glass {
								result = GlassComposer(c, nil, &draft, options)
							} else {
								result = Composer(c, &draft, options)
							}
							if result.Send {
								sends++
							}
							if result.Stop {
								stops++
							}
						}, width, 240)
						action := "Send"
						if running && draft == "" {
							action = "Stop"
						}
						button, found := tt.Find(action)
						trailing, ok := tt.Find("Details")
						if !found || !ok {
							t.Fatalf("missing actions: %v", tt.Texts())
						}
						if gap := button.X - trailing.X - trailing.W; math.Abs(float64(gap-8)) > .1 {
							t.Fatalf("trailing gap = %v, want 8: details=%+v action=%+v", gap, trailing, button)
						}
						if math.Abs(float64(button.Y+button.H/2-trailing.Y-trailing.H/2)) > .1 ||
							trailing.W != 24 || trailing.X < 10 || button.X+button.W > float32(width)-10 {
							t.Fatalf("bad trailing alignment: details=%+v action=%+v", trailing, button)
						}
						if err := tt.Click("Details"); err != nil || clicks != 1 || sends != 0 || stops != 0 {
							t.Fatalf("independent trailing action: %v clicks=%d sends=%d stops=%d", err, clicks, sends, stops)
						}
						if err := tt.Click(action); err != nil {
							t.Fatal(err)
						}
						if action == "Stop" && stops != 1 || draft != "" && sends != 1 || draft == "" && sends != 0 {
							t.Fatalf("action changed: sends=%d stops=%d", sends, stops)
						}
					})
				}
			}
		}
	}
}
