package gesture

import "github.com/ebitengine/purego/objc"

const eventTypeScrollWheel = 22

// currentEvent returns the AppKit event being dispatched, or 0 off the main
// thread, without AppKit or outside dispatch.
func currentEvent() objc.ID {
	app := objc.ID(objc.GetClass("NSApplication"))
	if app == 0 || !mainThread() {
		return 0
	}
	shared := app.Send(objc.RegisterName("sharedApplication"))
	if shared == 0 {
		return 0
	}
	return shared.Send(objc.RegisterName("currentEvent"))
}

// nativeScrollTouch reports whether the scroll being handled came from a
// touch surface: those report gesture and momentum phases, wheels none.
func nativeScrollTouch() (touch, ok bool) {
	ev := currentEvent()
	if ev == 0 || objc.Send[uint](ev, objc.RegisterName("type")) != eventTypeScrollWheel {
		return false, false
	}
	phase := objc.Send[uint](ev, objc.RegisterName("phase"))
	momentum := objc.Send[uint](ev, objc.RegisterName("momentumPhase"))
	return phase != 0 || momentum != 0, true
}

func mainThread() bool {
	thread := objc.ID(objc.GetClass("NSThread"))
	return thread != 0 && objc.Send[bool](thread, objc.RegisterName("isMainThread"))
}
