//go:build !darwin

package gesture

func nativeScrollTouch() (touch, ok bool) { return false, false }
