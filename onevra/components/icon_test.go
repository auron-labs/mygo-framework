package components_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestIconSources(t *testing.T) {
	wide := ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="24" viewBox="0 0 48 24"><path d="M2 2h44v20H2z" fill="currentColor"/></svg>`))
	for _, svg := range []*ui.SVG{lucide.Search(), wide, nil} {
		for _, dark := range []bool{false, true} {
			for _, scale := range []float32{1, 2} {
				t.Run(fmt.Sprintf("%p/%v/%g", svg, dark, scale), func(t *testing.T) {
					build := func(icon func(*ui.Context) ui.Element) *ui.Tester {
						tt := ui.NewTester(func(c *ui.Context) {
							c.SetTheme(theme.Palette(dark))
							ui.Row(c).FontSize(24).TextColor(c.Theme().Accent).Children(func() {
								icon(c).Label("inherited")
								icon(c).Size(32, 32).TextColor(c.Theme().TextMuted).Label("explicit")
							})
						}, 160, 80)
						tt.SetScale(scale)
						return tt
					}
					want := build(func(c *ui.Context) ui.Element { return ui.Icon(c, svg) })
					for _, create := range []func(*ui.Context) ui.Element{
						func(c *ui.Context) ui.Element { return components.Icon(c, svg) },
						func(c *ui.Context) ui.Element { return components.Icon(c, func() *ui.SVG { return svg }) },
					} {
						got := build(create)
						if !bytes.Equal(got.Image().Pix, want.Image().Pix) {
							t.Fatal("Icon changed SVG rendering or style inheritance")
						}
						for _, label := range []string{"inherited", "explicit"} {
							g, gok := got.Find(label)
							w, wok := want.Find(label)
							if !gok || !wok || g != w {
								t.Fatalf("%s bounds %v, want %v", label, g, w)
							}
						}
					}
				})
			}
		}
	}
}

func TestIconAccessor(t *testing.T) {
	calls, builds := 0, 0
	var empty func() *ui.SVG
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		ui.Row(c).Children(func() {
			components.Icon(c, func() *ui.SVG { calls++; return lucide.Search() }).Label("search")
			components.Icon(c, empty).Size(20, 20).Label("empty")
			components.Icon(c, lucide.Settings).Size(20, 20)
		})
	}, 100, 50)
	if calls != builds || calls == 0 {
		t.Fatalf("accessor called %d times for %d builds", calls, builds)
	}
	if r, ok := tt.Find("empty"); !ok || r.W != 20 || r.H != 20 {
		t.Fatalf("nil accessor lost element geometry: %v", r)
	}
}
