package timechart

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	rate      = 25
	plotLeft  = axisWidth + 6 // axis column and the plot's gap
	plotY     = 30
	rigWidth  = 900
	rigHeight = 460
)

var origin = float64(time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC).Unix())

// rig is two stacked charts linked by one Range over an hour of samples.
type rig struct {
	tt     *ui.Tester
	rng    *Range
	charts []*Chart
}

func newRig(t *testing.T) *rig {
	t.Helper()
	const n = 3600 * rate
	xs, ys, zs := make([]float64, n), make([]float64, n), make([]float64, n)
	for i := range xs {
		xs[i] = origin + float64(i)/rate
		ys[i] = math.Sin(float64(i) / 2000)
		zs[i] = float64(i % 100)
	}
	provider := func(vs []float64) Provider {
		return func(_ context.Context, from, to float64, w int) (Points, error) {
			return Decimate(xs, vs, from, to, w), nil
		}
	}
	r := &rig{rng: NewRange(origin, origin+3600)}
	r.charts = []*Chart{
		{
			Range: r.rng, Label: "Pulse", HideXAxis: true, Location: time.UTC, Series: []Series{{ID: "pulse", Label: "Pulse", Provider: provider(ys)}},
			Events: []Event{{Start: origin + 600, End: origin + 900, Label: "Arousal"}},
		},
		{Range: r.rng, Label: "Flow", Location: time.UTC, Series: []Series{{ID: "flow", Label: "Flow", Kind: Step, Provider: provider(zs)}}},
	}
	r.tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			for _, ch := range r.charts {
				ch.Build(c).Height(200)
			}
		})
	}, rigWidth, rigHeight)
	t.Cleanup(func() {
		for _, ch := range r.charts {
			ch.Close()
		}
	})
	r.tt.Frame()
	r.waitForData(t)
	return r
}

// waitForData frames until every series has points for the visible window.
func (r *rig) waitForData(t *testing.T) {
	t.Helper()
	eventually(t, "chart data", func() bool {
		r.tt.Frame()
		for _, ch := range r.charts {
			if !ch.Loaded() {
				return false
			}
		}
		return true
	})
}

// x is the window x of fraction f along the plot.
func (r *rig) x(f float64) float32 { return plotLeft + float32(f*float64(r.charts[0].width)) }

func TestWheelZoomsAroundTheCursor(t *testing.T) {
	r := newRig(t)
	r.rng.Set(origin+600, origin+2400)
	r.tt.Frame()
	lo, hi := r.rng.Visible()
	cursor := .25
	under := lo + cursor*(hi-lo)
	r.tt.Wheel(r.x(cursor), plotY, 0, -100)
	r.tt.Frame()
	nlo, nhi := r.rng.Visible()
	if nhi-nlo >= hi-lo {
		t.Fatalf("scrolling up did not zoom in: %v to %v", nlo, nhi)
	}
	if got := nlo + cursor*(nhi-nlo); math.Abs(got-under) > 1e-6*(hi-lo) {
		t.Fatalf("value under the cursor moved from %v to %v", under, got)
	}
	r.tt.Wheel(r.x(cursor), plotY, 0, 100)
	r.tt.Frame()
	if zlo, zhi := r.rng.Visible(); zhi-zlo <= nhi-nlo {
		t.Fatal("scrolling down did not zoom out")
	}
}

func TestPinchZooms(t *testing.T) {
	r := newRig(t)
	r.tt.Pinch(r.x(.5), plotY, 2)
	r.tt.Frame()
	lo, hi := r.rng.Visible()
	if math.Abs((hi-lo)-1800) > 1e-6 || math.Abs((lo+hi)/2-(origin+1800)) > 1e-6 {
		t.Fatalf("got %v to %v", lo, hi)
	}
}

func TestDragPansAndStopsAtTheData(t *testing.T) {
	r := newRig(t)
	r.rng.Set(origin+1000, origin+1600)
	r.tt.Frame()
	lo, hi := r.rng.Visible()
	width := float64(r.charts[0].width)
	r.tt.Press(r.x(.5), plotY)
	r.tt.Move(r.x(.5)-60, plotY)
	r.tt.Release(r.x(.5)-60, plotY)
	nlo, nhi := r.rng.Visible()
	want := 60 / width * (hi - lo)
	if math.Abs((nlo-lo)-want) > 1e-6 || math.Abs((nhi-nlo)-(hi-lo)) > 1e-6 {
		t.Fatalf("dragging left by 60 DIPs moved the view by %v, want %v", nlo-lo, want)
	}
	r.tt.Press(r.x(.1), plotY)
	r.tt.Move(r.x(.1)+5000, plotY)
	r.tt.Release(r.x(.1)+5000, plotY)
	if nlo, _ := r.rng.Visible(); nlo != origin {
		t.Fatalf("pan went past the start of the data: %v", nlo)
	}
}

func TestDoubleClickResets(t *testing.T) {
	r := newRig(t)
	r.rng.Set(origin+1000, origin+1100)
	r.tt.Frame()
	r.tt.ClickAt(r.x(.5), plotY)
	r.tt.ClickAt(r.x(.5), plotY)
	if lo, hi := r.rng.Visible(); lo != origin || hi != origin+3600 {
		t.Fatalf("got %v to %v", lo-origin, hi-origin)
	}
}

func TestInteractionWithOneChartMovesTheOther(t *testing.T) {
	r := newRig(t)
	// Only the second chart has an axis; zoom from the first.
	if !r.tt.HasText("23:00") {
		t.Fatalf("whole-hour axis missing: %v", r.tt.Texts())
	}
	r.tt.Wheel(r.x(.5), plotY, 0, -300)
	r.waitForData(t)
	lo, hi := r.rng.Visible()
	if hi-lo > 3600*.4 {
		t.Fatalf("wheel did not zoom: %v", hi-lo)
	}
	if r.tt.HasText("23:00") || !r.tt.HasText("22:30") {
		t.Fatalf("the linked chart's axis did not follow: %v", r.tt.Texts())
	}
}

func TestCrosshairIsSharedAcrossCharts(t *testing.T) {
	r := newRig(t)
	r.tt.Move(r.x(.5), plotY) // over the first chart
	r.tt.Frame()
	if x, ok := r.rng.Crosshair(); !ok || math.Abs(x-(origin+1800)) > 5 {
		t.Fatalf("crosshair %v %v", x, ok)
	}
	for _, label := range []string{"Pulse", "Flow"} {
		if !r.tt.HasText(label) {
			t.Fatalf("readout lacks %q: %v", label, r.tt.Texts())
		}
	}
	r.tt.Move(rigWidth-1, rigHeight-1) // off the charts
	r.tt.Frame()
	if _, ok := r.rng.Crosshair(); ok {
		t.Fatal("crosshair stayed after the pointer left")
	}
}

func TestEventFlagShowsOnlyWhileInView(t *testing.T) {
	r := newRig(t)
	if !r.tt.HasText("Arousal") {
		t.Fatalf("flag missing: %v", r.tt.Texts())
	}
	r.rng.Set(origin+2000, origin+2600)
	r.waitForData(t)
	if r.tt.HasText("Arousal") {
		t.Fatal("flag shown outside its interval")
	}
	r.rng.Set(origin+800, origin+1400)
	r.waitForData(t)
	if !r.tt.HasText("Arousal") {
		t.Fatal("flag of a partly visible interval missing")
	}
}

func TestKeyboardZoomsAndPans(t *testing.T) {
	r := newRig(t)
	if err := r.tt.Click("Pulse"); err != nil { // focus the first plot by its label
		t.Fatal(err)
	}
	r.tt.Key(0, ui.KeyEqual)
	lo, hi := r.rng.Visible()
	if math.Abs((hi-lo)-3600*keyZoom) > 1e-6 {
		t.Fatalf("got %v to %v", lo, hi)
	}
	r.tt.Key(0, ui.KeyRight)
	if nlo, _ := r.rng.Visible(); nlo <= lo {
		t.Fatal("right arrow did not pan")
	}
	r.tt.Key(0, ui.KeyHome)
	if lo, hi := r.rng.Visible(); lo != origin || hi != origin+3600 {
		t.Fatal("home did not reset")
	}
}

func TestReadAt(t *testing.T) {
	p := Points{X: []float64{0, 10, 20}, Y: []float64{0, 10, 0}}
	if v, _, ok := readAt(Line, p, 5, 1); !ok || v != 5 {
		t.Fatalf("line %v %v", v, ok)
	}
	if v, _, ok := readAt(Step, p, 15, 1); !ok || v != 10 {
		t.Fatalf("step %v %v", v, ok)
	}
	if lo, hi, ok := readAt(Band, p, 10, 11); !ok || lo != 0 || hi != 10 {
		t.Fatalf("band %v %v %v", lo, hi, ok)
	}
	if _, _, ok := readAt(Line, p, 30, 1); ok {
		t.Fatal("read past the last point")
	}
}
