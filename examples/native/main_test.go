package main

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestNativePreferenceControl(t *testing.T) {
	s := &screen{}
	tt := ui.NewTester(s.view, 640, 360)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if !s.compact {
		t.Fatal("checkbox did not update the retained preference")
	}
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if s.compact {
		t.Fatal("checkbox did not toggle back")
	}
}
