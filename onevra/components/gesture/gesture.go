// Package gesture turns scroll and touchpad input into pan and zoom for
// canvases such as graphs, maps and image viewers: a mouse wheel and a
// touchpad pinch zoom, and two fingers dragged on a touchpad pan.
//
// MyGo reports pinches as InputZoom, but scrolling without the device
// behind it. On macOS a narrow AppKit adapter reads the scroll phase of the
// event being handled, which only touch surfaces report; elsewhere precise
// scrolling counts as a touchpad.
package gesture

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Device is what produced a scroll.
type Device uint8

const (
	// Wheel is a mouse wheel, including smoothed ones that scroll by
	// pixels.
	Wheel Device = iota
	// Touchpad is a touch surface: a touchpad, or a Magic Mouse on macOS.
	Touchpad
)

// DeviceOf reports which device produced an InputScroll. Call it while
// handling the event, before returning from HandleInput.
func DeviceOf(ev ui.InputEvent) Device {
	if touch, ok := nativeScrollTouch(); ok {
		if touch {
			return Touchpad
		}
		return Wheel
	}
	if ev.Precise {
		return Touchpad
	}
	return Wheel
}

// Kind is the kind of a navigation Event.
type Kind uint8

const (
	// Pan moves the content by DX, DY DIPs: positive DX moves it right.
	Pan Kind = iota + 1
	// Zoom scales the content by Scale around X, Y.
	Zoom
)

// Event is a pan or zoom at X, Y, relative to the element's box.
type Event struct {
	Kind         Kind
	X, Y, DX, DY float32
	Scale        float64
	Device       Device
	// Pinch marks a zoom by a touchpad pinch, or Control-scroll on a
	// touchpad.
	Pinch bool
}

const (
	// wheelZoomDistance and pinchZoomDistance are the scroll distances in
	// DIPs that zoom by ×1.5.
	wheelZoomDistance = 120
	pinchZoomDistance = 48
)

// Classify maps a scroll or pinch to navigation. A pinch zooms; a touchpad
// pans unless Control or Command is held, which zooms as a pinch does; a
// wheel zooms, except a sideways wheel, or Shift with a vertical one on
// macOS, which pans across. Call it while handling the event; ok is false
// for other input.
func Classify(ev ui.InputEvent) (Event, bool) {
	if ev.Kind == ui.InputZoom {
		if ev.Scale <= 0 {
			return Event{}, false
		}
		return Event{Kind: Zoom, X: ev.X, Y: ev.Y, Scale: float64(ev.Scale), Device: Touchpad, Pinch: true}, true
	}
	if ev.Kind != ui.InputScroll {
		return Event{}, false
	}
	device := DeviceOf(ev)
	out := Event{X: ev.X, Y: ev.Y, Device: device}
	modified := ev.Mods&(ui.Ctrl|ui.Super) != 0
	switch {
	case device == Touchpad && !modified:
		out.Kind, out.DX, out.DY = Pan, -ev.DX, -ev.DY
	case device == Wheel && !modified && ev.DY == 0:
		out.Kind, out.DX = Pan, -ev.DX
	default:
		distance := float64(wheelZoomDistance)
		if device == Touchpad {
			distance, out.Pinch = pinchZoomDistance, true
		}
		out.Kind, out.Scale = Zoom, math.Exp(-float64(ev.DY)*math.Log(1.5)/distance)
	}
	return out, true
}

// Navigation delivers an element's pan and zoom gestures. Keep one per
// element; its zero value is ready to use.
type Navigation struct {
	fn func(Event)
}

// Handle has el take its input through fn, as HandleInput does, after
// sending scrolling and pinches to navigate. Navigation events reach
// navigate and are taken; other input goes to other, which may be nil.
// Call it on each build.
func (n *Navigation) Handle(c *ui.Context, el ui.Element, navigate func(Event), other func(ui.InputEvent) bool) ui.Element {
	n.fn = navigate
	return el.HandleInput(func(ev ui.InputEvent) bool {
		if g, ok := Classify(ev); ok && n.fn != nil {
			n.fn(g)
			return true
		}
		if other != nil {
			return other(ev)
		}
		return false
	})
}
