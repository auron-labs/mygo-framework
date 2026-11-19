// Package phone tells the phone systems from the desktop ones, for the parts
// of a shell the two draw differently: a phone's system owns its window's
// chrome, so the shell draws no title bar and keeps content off the bars.
package phone

import "runtime"

// GOOS is the system the app runs on. It is a variable so tests on a desktop
// can exercise the phone layout.
var GOOS = runtime.GOOS

// On reports whether the app runs on a phone's system, iOS's or Android's,
// whose windows have no title bar of the app's own and are covered at their
// edges by the system's bars. Linux's and Windows's touchscreens are desktops
// and keep their windows' chrome.
func On() bool {
	return GOOS == "ios" || GOOS == "android"
}
