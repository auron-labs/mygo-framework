package state_test

import (
	"mygo-framework/state"
	"slices"
	"testing"
)

type settings struct {
	Theme string
	Size  int
	Tags  []string
}

func TestDerivedRecomputesOnlyWhenSourceChanges(t *testing.T) {
	cell := state.NewCell(settings{Theme: "light"})
	computed := 0
	theme := state.Select(cell, func(s settings) string {
		computed++
		return s.Theme
	})
	for range 3 {
		if theme.Get() != "light" {
			t.Fatal("wrong value")
		}
	}
	if computed != 1 {
		t.Fatalf("computed %d times without a change", computed)
	}
	cell.Update(func(s *settings) { s.Theme = "dark" })
	if theme.Get() != "dark" || computed != 2 {
		t.Fatalf("theme = %q after %d computations", theme.Get(), computed)
	}
}

func TestDerivedSubscribeSuppressesEqualValues(t *testing.T) {
	cell := state.NewCell(settings{Theme: "light"})
	var got []string
	state.Select(cell, func(s settings) string { return s.Theme }).Subscribe(func(v string) { got = append(got, v) })

	cell.Update(func(s *settings) { s.Size = 1 })
	cell.Update(func(s *settings) { s.Theme = "light" })
	cell.Update(func(s *settings) { s.Theme = "dark" })
	cell.Update(func(s *settings) { s.Size = 2 })
	cell.Update(func(s *settings) { s.Theme = "light" })

	if want := []string{"dark", "light"}; !slices.Equal(got, want) {
		t.Fatalf("delivered %v, want %v", got, want)
	}
}

func TestSelectFuncAndChaining(t *testing.T) {
	cell := state.NewCell(settings{Tags: []string{"a"}})
	tags := state.SelectFunc(cell, func(s settings) []string { return s.Tags }, slices.Equal)
	count := state.Select(tags, func(tags []string) int { return len(tags) })
	var got []int
	count.Subscribe(func(n int) { got = append(got, n) })

	cell.Update(func(s *settings) { s.Tags = []string{"a"} })
	cell.Update(func(s *settings) { s.Tags = []string{"a", "b"} })
	cell.Update(func(s *settings) { s.Tags = []string{"c", "d"} })

	if !slices.Equal(got, []int{2}) || count.Get() != 2 {
		t.Fatalf("delivered %v, count = %d", got, count.Get())
	}
}
