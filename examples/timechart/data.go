package main

import (
	"context"
	"math"
	"math/rand/v2"
	"mygo-framework/timechart"
	"time"
)

const (
	sampleRate = 25 // samples per second per channel
	nightHours = 8
	samples    = nightHours * 3600 * sampleRate
)

// recording is a synthetic night of three channels sampled at sampleRate.
type recording struct {
	start                 float64 // Unix seconds of the first sample
	times                 []float64
	airflow, spo2, pulse  []float64
	apneas, arousalEvents []timechart.Event
}

// synthesize builds a deterministic night: breathing that stops now and
// then, oxygen saturation that dips after each stop, and a pulse that
// rises at each arousal.
func synthesize(start time.Time) *recording {
	rng := rand.New(rand.NewPCG(7, 25)) //nolint:gosec // reproducible synthetic data
	r := &recording{
		start: float64(start.UnixNano()) / 1e9,
		times: make([]float64, samples), airflow: make([]float64, samples),
		spo2: make([]float64, samples), pulse: make([]float64, samples),
	}
	var dips []float64 // seconds into the night where an apnea ends
	for at := 300.0; at < nightHours*3600-300; at += 240 + rng.Float64()*600 {
		length := 12 + rng.Float64()*35
		r.apneas = append(r.apneas, timechart.Event{Start: r.start + at, End: r.start + at + length, Label: "Apnea"})
		dips = append(dips, at+length)
		if rng.Float64() < .6 {
			r.arousalEvents = append(r.arousalEvents, timechart.Event{Start: r.start + at + length + 2, Label: "Arousal"})
		}
	}
	apnea, dip, arousal := 0, 0, 0
	for i := range samples {
		t := float64(i) / sampleRate
		r.times[i] = r.start + t
		for apnea < len(r.apneas) && r.apneas[apnea].End-r.start < t {
			apnea++
		}
		breath := 1 + .3*math.Sin(t/700)
		if apnea < len(r.apneas) && r.apneas[apnea].Start-r.start <= t {
			breath = .04
		}
		r.airflow[i] = breath*math.Sin(2*math.Pi*.27*t) + .05*(rng.Float64()-.5)

		for dip < len(dips) && dips[dip]+40 < t {
			dip++
		}
		saturation := 96 + 1.2*math.Sin(t/1500)
		if dip < len(dips) && t > dips[dip]-30 {
			saturation -= 6 * math.Exp(-math.Abs(t-dips[dip]-8)/14)
		}
		r.spo2[i] = math.Round(saturation)

		for arousal < len(r.arousalEvents) && r.arousalEvents[arousal].Start-r.start+25 < t {
			arousal++
		}
		rise := 0.0
		if arousal < len(r.arousalEvents) {
			rise = 14 * math.Exp(-math.Abs(t-(r.arousalEvents[arousal].Start-r.start)-6)/7)
		}
		r.pulse[i] = 58 + rise + 5*math.Sin(t/900) + 9*math.Sin(2*math.Pi*1.05*t) + 2*(rng.Float64()-.5)
	}
	return r
}

// provider serves one channel to a chart from memory.
func (r *recording) provider(values []float64) timechart.Provider {
	return func(_ context.Context, from, to float64, pixelWidth int) (timechart.Points, error) {
		return timechart.Decimate(r.times, values, from, to, pixelWidth), nil
	}
}
