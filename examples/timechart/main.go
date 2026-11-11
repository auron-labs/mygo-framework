// Timechart demonstrates linked time-series charts over a night of
// 25 Hz recordings.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/auron-labs/mygo-framework/timechart"

	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/shells/plain"
	"github.com/oneo-me/onevra/source/theme"
)

const (
	appName     = "Framework Time Chart"
	chartHeight = 220
)

// screen stacks one chart per channel over a shared time range.
type screen struct {
	charts []*timechart.Chart
}

func newScreen(r *recording) *screen {
	rng := timechart.NewRange(r.start, r.start+nightHours*3600)
	return &screen{charts: []*timechart.Chart{
		{
			Range: rng, Label: "Airflow", Height: chartHeight, HideXAxis: true, Events: r.apneas,
			Series:  []timechart.Series{{ID: "airflow", Label: "Airflow", Provider: r.provider(r.airflow)}},
			FormatY: func(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) },
		},
		{
			Range: rng, Label: "Oxygen saturation", Height: chartHeight, HideXAxis: true, YMin: 80, YMax: 100,
			Series:  []timechart.Series{{ID: "spo2", Label: "Oxygen", Kind: timechart.Step, Provider: r.provider(r.spo2)}},
			FormatY: func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "%" },
		},
		{
			Range: rng, Label: "Pulse", Height: chartHeight, Events: r.arousalEvents,
			Series:  []timechart.Series{{ID: "pulse", Label: "Pulse", Kind: timechart.Band, Provider: r.provider(r.pulse)}},
			FormatY: func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + " bpm" },
		},
	}}
}

func (s *screen) view(c *ui.Context) {
	ui.Column(c).Fill().Padding(16).Gap(8).Children(func() {
		components.Text(c, "Overnight recording").FontSize(20).Bold()
		for _, chart := range s.charts {
			chart.Build(c)
		}
	})
}

// loaded reports whether every chart has points for its visible window.
func (s *screen) loaded() bool {
	for _, chart := range s.charts {
		if !chart.Loaded() {
			return false
		}
	}
	return true
}

func (s *screen) close() {
	for _, chart := range s.charts {
		chart.Close()
	}
}

func newRecording() *recording {
	return synthesize(time.Date(2026, 3, 1, 22, 30, 0, 0, time.Local))
}

func main() {
	mcpAddress := flag.String("mcp", "", "Serve an isolated headless UI over MCP at a loopback IP:port instead of opening a window")
	flag.Parse()
	if *mcpAddress != "" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		err := mygomcp.Run(ctx, *mcpAddress, []mygomcp.Fixture{fixture()})
		stop()
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	s := newScreen(newRecording())
	defer s.close()
	mygo.App.SetName(appName)
	mygo.App.WhenReady(func() {
		shell := plain.NewShell(plain.ShellOptions{
			Title: appName, Width: 1000, Height: 760, MinWidth: 560, MinHeight: 520,
			StateKey: "main", Theme: theme.Onevra, Content: s.view,
			Hidden: true,
		})
		win := shell.Window()
		win.OnClosed(func() { mygo.App.Quit() })
		win.OnReadyToShow(win.Show)
	})
	if err := mygo.App.Run(); err != nil {
		return fmt.Errorf("run time chart: %w", err)
	}
	return nil
}
