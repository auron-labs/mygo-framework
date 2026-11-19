package components

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func carouselTester(state *CarouselState, options CarouselOptions, changes *int) *ui.Tester {
	options.Label = "Suites"
	if options.Count == 0 {
		options.Count = 5
	}
	options.Slide = func(c *ui.Context, i int) {
		ui.Box(c).FillWidth().Height(120).Label(fmt.Sprintf("slide %d", i))
	}
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Column(c).Fill().Children(func() {
			if Carousel(c, state, options).Changed() && changes != nil {
				*changes++
			}
		})
	}, 600, 400)
}

func settleCarousel(tt *ui.Tester) {
	for range 4 {
		tt.Advance(400 * time.Millisecond)
		tt.Frame()
	}
}

// centered reports whether slide i rests in the middle of the 600-DIP view.
func centered(tt *ui.Tester, i int) bool {
	r, ok := tt.Find(fmt.Sprintf("slide %d", i))
	return ok && math.Abs(float64(r.X+r.W/2-300)) < 1
}

func TestCarouselCentersTheActiveSlide(t *testing.T) {
	state := &CarouselState{}
	tt := carouselTester(state, CarouselOptions{}, nil)
	settleCarousel(tt)
	first, _ := tt.Find("slide 0")
	if !centered(tt, 0) || first.W != 340 {
		t.Fatalf("first slide not centered at its widest: %+v", first)
	}
	// The neighbor peeks in, shrunk around its own center one step away.
	next, _ := tt.Find("slide 1")
	// It shows clipped by the view's edge, from its left at a tenth smaller.
	if math.Abs(float64(next.X-(300+356-340*.9/2))) > 1 || next.W >= first.W {
		t.Fatalf("neighbor not a smaller slide one step right: %+v", next)
	}
	// Slides take at most 80% of a narrow carousel.
	tt.SetSize(300, 400)
	settleCarousel(tt)
	if r, _ := tt.Find("slide 0"); r.W != 240 || math.Abs(float64(r.X+r.W/2-150)) > 1 {
		t.Fatalf("narrow carousel slide: %+v", r)
	}
}

func TestCarouselArrowsMoveAndAnnounce(t *testing.T) {
	state := &CarouselState{}
	changes := 0
	tt := carouselTester(state, CarouselOptions{}, &changes)
	settleCarousel(tt)
	if err := tt.Click("Next slide"); err != nil {
		t.Fatal(err)
	}
	settleCarousel(tt)
	if state.Index != 1 || changes != 1 || !centered(tt, 1) {
		t.Fatalf("next: index %d, %d changes, centered %v", state.Index, changes, centered(tt, 1))
	}
	if !slices.Contains(tt.Announcements(), "2 of 5") {
		t.Fatalf("announcements %q", tt.Announcements())
	}
	if err := tt.Click("Previous slide"); err != nil {
		t.Fatal(err)
	}
	// The previous arrow stays at the first slide, doing nothing.
	_ = tt.Click("Previous slide")
	settleCarousel(tt)
	if state.Index != 0 || !centered(tt, 0) {
		t.Fatalf("previous: index %d", state.Index)
	}
	// The indicators are tabs after the slide area.
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyEnd)
	settleCarousel(tt)
	if state.Index != 4 || !centered(tt, 4) {
		t.Fatalf("indicator: index %d", state.Index)
	}
}

func TestCarouselFollowsTheApplication(t *testing.T) {
	state := &CarouselState{}
	changes := 0
	tt := carouselTester(state, CarouselOptions{}, &changes)
	settleCarousel(tt)
	state.Index = 4
	settleCarousel(tt)
	if !centered(tt, 4) || changes != 0 {
		t.Fatalf("application index: centered %v, %d changes", centered(tt, 4), changes)
	}
	state.Index = 9
	tt.Frame()
	if state.Index != 4 {
		t.Fatalf("index not held within the slides: %d", state.Index)
	}
}

func TestCarouselDragging(t *testing.T) {
	state := &CarouselState{}
	tt := carouselTester(state, CarouselOptions{}, nil)
	settleCarousel(tt)
	drag := func(from, to float32, steps int, per time.Duration) {
		tt.Move(from, 60)
		tt.Press(from, 60)
		for i := 1; i <= steps; i++ {
			tt.Advance(per)
			tt.Move(from+(to-from)*float32(i)/float32(steps), 60)
		}
		// Held still before letting go: no flick.
		if per > 0 {
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
		}
		tt.Release(to, 60)
		settleCarousel(tt)
	}
	// A slow drag short of half a step settles back.
	drag(400, 300, 10, 30*time.Millisecond)
	if state.Index != 0 || !centered(tt, 0) {
		t.Fatalf("short drag moved to %d", state.Index)
	}
	// Past half a step, it moves on.
	drag(500, 250, 10, 30*time.Millisecond)
	if state.Index != 1 || !centered(tt, 1) {
		t.Fatalf("long drag rests on %d", state.Index)
	}
	// A quick flick moves a slide however short.
	tt.Move(400, 60)
	tt.Press(400, 60)
	for i := 1; i <= 4; i++ {
		tt.Advance(8 * time.Millisecond)
		tt.Move(400-float32(i)*15, 60)
	}
	tt.Release(340, 60)
	settleCarousel(tt)
	if state.Index != 2 {
		t.Fatalf("flick rests on %d", state.Index)
	}
	// A click on a neighbor centers it.
	tt.ClickAt(80, 60)
	settleCarousel(tt)
	if state.Index != 1 || !centered(tt, 1) {
		t.Fatalf("clicked neighbor: %d", state.Index)
	}
}

func TestCarouselSwipe(t *testing.T) {
	state := &CarouselState{}
	tt := carouselTester(state, CarouselOptions{}, nil)
	settleCarousel(tt)
	for range 10 {
		tt.Advance(10 * time.Millisecond)
		tt.Scroll(300, 60, 25, 0)
	}
	settleCarousel(tt)
	if state.Index != 1 || !centered(tt, 1) {
		t.Fatalf("swipe rests on %d", state.Index)
	}
	// A vertical scroll leaves the slides alone.
	tt.Scroll(300, 60, 0, 200)
	settleCarousel(tt)
	if state.Index != 1 {
		t.Fatalf("vertical scroll moved to %d", state.Index)
	}
}

func TestCarouselKeyboard(t *testing.T) {
	state := &CarouselState{}
	tt := carouselTester(state, CarouselOptions{}, nil)
	settleCarousel(tt)
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Slides") {
		t.Fatal("Tab does not focus the slide area")
	}
	tt.Key(0, ui.KeyRight)
	if state.Index != 1 {
		t.Fatalf("right arrow: %d", state.Index)
	}
	tt.Key(0, ui.KeyEnd)
	if state.Index != 4 {
		t.Fatalf("End: %d", state.Index)
	}
	tt.Key(0, ui.KeyHome)
	settleCarousel(tt)
	if state.Index != 0 || !centered(tt, 0) {
		t.Fatalf("Home: %d", state.Index)
	}
}

func TestCarouselRotation(t *testing.T) {
	state := &CarouselState{Playing: true}
	tt := carouselTester(state, CarouselOptions{Count: 3, Interval: time.Second}, nil)
	tt.Frame()
	run := func(d time.Duration) {
		for range int(d / (50 * time.Millisecond)) {
			tt.Advance(50 * time.Millisecond)
			tt.Frame()
		}
	}
	run(1100 * time.Millisecond)
	if state.Index != 1 {
		t.Fatalf("rotation after an interval: %d", state.Index)
	}
	// Hovering holds the slide.
	tt.Move(300, 60)
	run(1500 * time.Millisecond)
	if state.Index != 1 {
		t.Fatalf("rotated while hovered: %d", state.Index)
	}
	tt.Move(300, 390)
	run(2100 * time.Millisecond)
	if state.Index != 0 {
		t.Fatalf("rotation wraps to the first slide: %d", state.Index)
	}
	if err := tt.Click("Pause slide rotation"); err != nil {
		t.Fatal(err)
	}
	tt.Move(300, 390)
	run(2 * time.Second)
	if state.Playing || state.Index != 0 {
		t.Fatalf("paused: playing %v, index %d", state.Playing, state.Index)
	}
	if _, ok := tt.Find("Start slide rotation"); !ok {
		t.Fatal("no play control while paused")
	}
	// Reduced Motion leaves out rotation.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if _, ok := tt.Find("Start slide rotation"); ok {
		t.Fatal("play control shown with Reduced Motion")
	}
}

func TestCarouselSingleSlide(t *testing.T) {
	state := &CarouselState{}
	tt := carouselTester(state, CarouselOptions{Count: 1, Interval: time.Second}, nil)
	settleCarousel(tt)
	if !centered(tt, 0) {
		t.Fatal("single slide not centered")
	}
	for _, name := range []string{"Next slide", "Start slide rotation", "Choose a slide"} {
		if _, ok := tt.Find(name); ok {
			t.Fatalf("single slide shows %q", name)
		}
	}
}
