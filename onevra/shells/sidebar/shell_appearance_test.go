package sidebar

import (
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestShellTheme(t *testing.T) {
	custom := ui.LightTheme()
	custom.Background = ui.Hex("#123456")
	for _, tc := range []struct {
		name  string
		theme func(*ui.Context) *ui.Theme
		dark  bool
		want  ui.Color
	}{
		{"MyGo default", nil, false, ui.LightTheme().Background},
		{"MyGo default dark", nil, true, ui.DarkTheme().Background},
		{"Onevra", theme.Onevra, false, theme.LightPalette().Background},
		{"Onevra dark", theme.Onevra, true, theme.DarkPalette().Background},
		{"custom", func(*ui.Context) *ui.Theme { return custom }, false, custom.Background},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var content ui.Color
			s := newShell(ShellOptions{Theme: tc.theme, Content: func(c *ui.Context) { content = c.Theme().Background }})
			tt := ui.NewTester(s.view, 400, 300)
			tt.SetDark(tc.dark)
			if content != tc.want {
				t.Errorf("content background = %v, want %v", content, tc.want)
			}
			// Shell paints its own opaque surface with the same theme.
			if p := tt.Image().RGBAAt(399, 299); p.R != tc.want.R || p.G != tc.want.G || p.B != tc.want.B {
				t.Errorf("shell background = %v, want %v", p, tc.want)
			}
		})
	}
}
