package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

const (
	// railBreakpoint is the window width below which the sidebar shrinks to an
	// icon-only rail. The shell has no breakpoint of its own.
	railBreakpoint = 720
	// railWidth is the rail's width, and the shell's minimum sidebar width.
	railWidth = 64
	// defaultLeftWidth is the sidebar's width until the user drags it.
	defaultLeftWidth = 240
	// The window's default and minimum sizes. The minimum is narrower than
	// railBreakpoint so the rail is reachable.
	defaultWidth, defaultHeight = 960, 640
	minWidth, minHeight         = 520, 480
)

// leftHost is the part of sidebar.Shell that sizes the sidebar. The window
// passes its shell; headless views pass staticLeft.
type leftHost interface {
	LeftWidth() float32
	SetLeftWidth(width float32)
}

// staticLeft is a leftHost without a window, for the headless views.
type staticLeft struct{ width float32 }

func (h *staticLeft) LeftWidth() float32     { return h.width }
func (h *staticLeft) SetLeftWidth(w float32) { h.width = w }

// syncRail reports whether the sidebar shows as an icon rail at the current
// window width, and resizes the sidebar to match. It remembers the width the
// user chose when the rail takes over and restores it when the window widens
// again. It only calls SetLeftWidth when the width must change, since each call
// requests another frame.
func (s *screen) syncRail(c *ui.Context) (rail bool) {
	width, _ := c.Size()
	rail = width < railBreakpoint
	if s.host == nil {
		return rail
	}
	current := s.host.LeftWidth()
	switch {
	case rail && !s.railActive:
		s.railActive = true
		if current > railWidth {
			s.expandedWidth = current
		}
		s.host.SetLeftWidth(railWidth)
	case rail && !sameWidth(current, railWidth):
		// The rail has a fixed width; undo a drag of the separator.
		s.host.SetLeftWidth(railWidth)
	case !rail && s.railActive:
		s.railActive = false
		if s.expandedWidth <= railWidth {
			s.expandedWidth = defaultLeftWidth
		}
		s.host.SetLeftWidth(s.expandedWidth)
	}
	return rail
}

func sameWidth(a, b float32) bool { return math.Abs(float64(a-b)) < 0.5 }
