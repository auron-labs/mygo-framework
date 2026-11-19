package components

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSkeletonOpacity(t *testing.T) {
	start := time.UnixMilli(0)
	if got := skeletonOpacity(start, 0); got != 1 {
		t.Fatalf("beat start = %v, want 1", got)
	}
	if got := skeletonOpacity(start.Add(skeletonPulse/2), 0); got < .519 || got > .521 {
		t.Fatalf("beat middle = %v, want .52", got)
	}
	// Each block reaches a phase one stagger after the block above it.
	if a, b := skeletonOpacity(start.Add(500*time.Millisecond), 0), skeletonOpacity(start.Add(500*time.Millisecond+skeletonStagger), 1); a != b {
		t.Fatalf("staggered block = %v, want %v", b, a)
	}
	for ms := int64(0); ms < 2*skeletonPulse.Milliseconds(); ms += 37 {
		if got := skeletonOpacity(start.Add(time.Duration(ms)*time.Millisecond), 3); got < .52 || got > 1 {
			t.Fatalf("opacity at %dms = %v", ms, got)
		}
	}
}

func TestSkeletonLayout(t *testing.T) {
	var space func(float32) float32
	height := func(options SkeletonOptions) float32 {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			space = c.Theme().Space
			ui.Column(c).Fill().AlignItems(ui.Start).Children(func() {
				ui.Column(c).Width(240).Children(func() { Skeleton(c, options) })
			})
		}, 300, 300)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		r, ok := tt.Find("Loading content")
		if !ok || r.W != 240 {
			t.Fatalf("%+v: status bounds = %+v", options, r)
		}
		return r.H
	}
	lines := func(n float32) float32 { return space(1) + space(4) + (n-1)*(space(3)+space(3)) }
	for _, tc := range []struct {
		options SkeletonOptions
		want    float32
	}{
		{SkeletonOptions{}, 3},
		{SkeletonOptions{Lines: 1}, 1},
		{SkeletonOptions{Lines: -2}, 1},
		{SkeletonOptions{Lines: 6}, 6},
		{SkeletonOptions{Lines: 20}, 6},
	} {
		if got := height(tc.options); got != lines(tc.want) {
			t.Errorf("%+v: height = %v, want %v lines (%v)", tc.options, got, tc.want, lines(tc.want))
		}
	}
	if got := height(SkeletonOptions{Lines: 1, Avatar: true}); got != space(9) {
		t.Errorf("avatar height = %v, want %v", got, space(9))
	}
}

func TestSkeletonLabel(t *testing.T) {
	label := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Skeleton(c, SkeletonOptions{Label: label})
	}, 200, 100)
	tt.SetLocale("zh-Hans")
	if _, ok := tt.Find("正在加载内容"); !ok {
		t.Fatalf("missing localized status label: %v", tt.Texts())
	}
	label = "Loading profile"
	tt.Frame()
	if _, ok := tt.Find(label); !ok {
		t.Fatalf("missing custom status label: %v", tt.Texts())
	}
}

func TestSkeletonPulse(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				Skeleton(c, SkeletonOptions{Avatar: true})
			}, 160, 80)
			frame := func(reduce bool) []byte {
				tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
				before := append([]byte(nil), tt.Image().Pix...)
				tt.Advance(skeletonPulse / 2)
				if reduce != bytes.Equal(before, tt.Image().Pix) {
					t.Fatalf("reduced motion %v: frames equal = %v", reduce, !reduce)
				}
				return before
			}
			frame(false)
			frame(true)
		})
	}
}

func TestSkeletonContent(t *testing.T) {
	loading := true
	var space func(float32) float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		space = c.Theme().Space
		ui.Column(c).Fill().Children(func() {
			SkeletonContent(c, loading, SkeletonOptions{Lines: 1}, func() {
				ui.Box(c).Height(120).FillWidth().Label("profile")
			}).Label("frame")
		})
	}, 200, 200)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if _, ok := tt.Find("Loading content"); !ok {
		t.Fatal("missing placeholder while loading")
	}
	if _, ok := tt.Find("profile"); ok {
		t.Fatal("content built while loading")
	}
	if r, _ := tt.Find("frame"); r.H != space(5) {
		t.Fatalf("loading height = %v, want %v", r.H, space(5))
	}

	tt.SetPreferences(ui.Preferences{})
	loading = false
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	if r, _ := tt.Find("frame"); r.H <= space(5) || r.H >= 120 {
		t.Fatalf("height mid-spring = %v, want between %v and 120", r.H, space(5))
	}
	tt.Advance(2 * time.Second)
	if r, _ := tt.Find("frame"); r.H != 120 {
		t.Fatalf("settled height = %v, want 120", r.H)
	}
	if _, ok := tt.Find("profile"); !ok {
		t.Fatal("missing loaded content")
	}
	if _, ok := tt.Find("Loading content"); ok {
		t.Fatal("placeholder remains after loading")
	}

	// Reduced Motion swaps without moving the height.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	loading = true
	tt.Frame()
	if r, _ := tt.Find("frame"); r.H != space(5) {
		t.Fatalf("reduced-motion height = %v, want %v", r.H, space(5))
	}
}
