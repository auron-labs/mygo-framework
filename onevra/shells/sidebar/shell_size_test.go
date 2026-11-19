package sidebar

import "testing"

func TestShellWindowSize(t *testing.T) {
	for _, tt := range []struct {
		name    string
		options ShellOptions
		want    [4]int
	}{
		{"defaults", ShellOptions{}, [4]int{820, 740, 820, 740}},
		{"negative", ShellOptions{Width: -1, Height: -1, MinWidth: -1, MinHeight: -1}, [4]int{820, 740, 820, 740}},
		{"larger initial", ShellOptions{Width: 1200, Height: 900}, [4]int{1200, 900, 820, 740}},
		{"smaller initial", ShellOptions{Width: 400, Height: 300}, [4]int{820, 740, 820, 740}},
		{"explicit smaller minimum", ShellOptions{Width: 400, Height: 300, MinWidth: 360, MinHeight: 240}, [4]int{400, 300, 360, 240}},
		{"larger minimum", ShellOptions{MinWidth: 1000, MinHeight: 800}, [4]int{1000, 800, 1000, 800}},
		{"independent dimensions", ShellOptions{Width: 1000, MinHeight: 480}, [4]int{1000, 740, 820, 480}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			o := newShell(tt.options).nativeOptions()
			if got := [4]int{o.Width, o.Height, o.MinWidth, o.MinHeight}; got != tt.want {
				t.Fatalf("initial/minimum dimensions = %v, want %v", got, tt.want)
			}
		})
	}
}
