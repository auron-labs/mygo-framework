package mygomcp

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

type touchState struct {
	taps, longs int
	scroll      ui.ScrollState
	safe        [4]float32
}

func touchFixture(st *touchState) Fixture {
	return Fixture{Name: "touch", Width: 200, Height: 400, Scale: 1, Factory: func() (func(*ui.Context), func(), error) {
		return func(c *ui.Context) {
			st.safe[0], st.safe[1], st.safe[2], st.safe[3] = c.SafeArea()
			ui.Column(c).Fill().Children(func() {
				ui.Scroll(c).Height(150).TrackScroll(&st.scroll).Children(func() {
					ui.Column(c).Height(800).Shrink(0).Children(func() {
						ui.Box(c).Size(150, 600).Shrink(0)
					})
				})
				b := ui.Box(c).Size(150, 60).Shrink(0)
				if b.LongPressed() {
					st.longs++
				}
				if b.Clicked() {
					st.taps++
				}
			})
		}, nil, nil
	}}
}

func TestTouchTools(t *testing.T) {
	var st touchState
	s := testService(t, touchFixture(&st))
	cs := testClient(t, s)
	id := s.sessions[0].id
	read := func() touchState {
		v, err := s.call(t.Context(), id, func(*session) (any, error) { return st, nil })
		if err != nil {
			t.Fatal(err)
		}
		return v.(touchState)
	}
	at := func(y float64) map[string]any {
		return map[string]any{"session_id": id, "observation_id": inspect(t, cs, id).ObservationID, "x": 50, "y": y}
	}

	callTool(t, cs, "mygo_tap", at(180), nil)
	if got := read(); got.taps != 1 || got.longs != 0 {
		t.Fatalf("tap: %+v", got)
	}

	args := at(180)
	var out status
	callTool(t, cs, "mygo_long_press", args, &out)
	if got := read(); got.longs != 1 || got.taps != 1 {
		t.Fatalf("long press: %+v", got)
	}
	args = at(180)
	args["hold_ms"] = 100
	callTool(t, cs, "mygo_long_press", args, nil)
	if got := read(); got.longs != 1 || got.taps != 2 {
		t.Fatalf("short press should tap: %+v", got)
	}

	// A finger dragged up over the scroll area pans it down, unlike a mouse drag.
	args = at(100)
	args["to_x"], args["to_y"], args["duration_ms"], args["hold_ms"] = 50, 20, 200, 300
	callTool(t, cs, "mygo_drag", args, nil)
	if got := read(); got.scroll.Y < 70 || got.scroll.Y > 80 {
		t.Fatalf("drag scroll: %+v", got.scroll)
	}

	// Lifting while still moving flings the content on as time advances.
	args = at(100)
	args["to_x"], args["to_y"], args["duration_ms"] = 50, 60, 100
	callTool(t, cs, "mygo_drag", args, nil)
	flung := read().scroll.Y
	callTool(t, cs, "mygo_advance", map[string]any{"session_id": id, "milliseconds": 300}, nil)
	if got := read().scroll.Y; got <= flung {
		t.Fatalf("no fling: %v then %v", flung, got)
	}

	// Cancel ends the gesture without a fling or tap.
	before := read()
	args = at(100)
	args["to_x"], args["to_y"], args["duration_ms"], args["cancel"] = 50, 60, 100, true
	callTool(t, cs, "mygo_drag", args, nil)
	if got := read(); got.taps != before.taps {
		t.Fatalf("cancel tapped: %+v", got)
	}

	// Touch tools require a fresh observation and bounded input.
	targetingError(t, cs, "mygo_tap", map[string]any{"session_id": id, "observation_id": args["observation_id"], "x": 50, "y": 100}, "stale_observation")
	for name, bad := range map[string]map[string]any{
		"mygo_long_press": {"hold_ms": 1001},
		"mygo_drag":       {"to_x": 50, "to_y": 20, "duration_ms": -1},
	} {
		in := at(100)
		for k, v := range bad {
			in[k] = v
		}
		toolError(t, cs, name, in)
	}
	in := at(100)
	in["to_x"], in["to_y"], in["duration_ms"] = 50, 9999, 0
	targetingError(t, cs, "mygo_drag", in, "out_of_bounds")
}

func TestSetViewport(t *testing.T) {
	var st touchState
	s := testService(t, touchFixture(&st), demoFixture("other", 1))
	cs := testClient(t, s)
	id := s.sessions[0].id
	obs := inspect(t, cs, id)

	var info viewportInfo
	callTool(t, cs, "mygo_set_viewport", map[string]any{"session_id": id, "width": 390, "height": 844, "scale": 2, "inset_top": 47, "inset_bottom": 34}, &info)
	if info.ContentWidth != 390 || info.ContentHeight != 844 || info.Scale != 2 {
		t.Fatalf("geometry: %+v", info)
	}
	if got := st.safe; got != [4]float32{47, 0, 34, 0} {
		t.Fatalf("safe area: %v", got)
	}
	targetingError(t, cs, "mygo_tap", map[string]any{"session_id": id, "observation_id": obs.ObservationID, "x": 10, "y": 10}, "stale_observation")

	shot := inspect(t, cs, id)
	if shot.ContentWidth != 390 || shot.ImageWidth != 780 || shot.ImageHeight != 1688 {
		t.Fatalf("observation after resize: %+v", shot)
	}
	var list struct{ Sessions []sessionInfo }
	callTool(t, cs, "mygo_sessions", struct{}{}, &list)
	if list.Sessions[0].Width != 390 || list.Sessions[0].Scale != 2 {
		t.Fatalf("session info: %+v", list.Sessions[0])
	}
	// Taps map through the new geometry, and later calls reset the insets.
	callTool(t, cs, "mygo_set_viewport", map[string]any{"session_id": id, "width": 390, "height": 844}, &info)
	if info.Scale != 2 || st.safe != [4]float32{} {
		t.Fatalf("scale kept and insets reset: %+v %v", info, st.safe)
	}

	for _, bad := range []map[string]any{
		{"width": 0, "height": 100},
		{"width": 5000, "height": 100},
		{"width": 100, "height": 100, "scale": 5},
		{"width": 100, "height": 100, "inset_top": 60, "inset_bottom": 40},
		{"width": 100, "height": 100, "inset_left": -1},
		{"width": 2000, "height": 2000, "scale": 4},
	} {
		bad["session_id"] = id
		toolError(t, cs, "mygo_set_viewport", bad)
	}
	callTool(t, cs, "mygo_set_viewport", map[string]any{"session_id": id, "width": 390, "height": 844}, nil)
}
