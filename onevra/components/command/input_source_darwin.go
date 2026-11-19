package command

import (
	"fmt"

	"github.com/ebitengine/purego/objc"
)

// AppKit owns input-source selection; this adapter does not intercept keys,
// enable sources, discard marked text, or implement an input method.
func nativeInputContext() objc.ID {
	thread := objc.GetClass("NSThread")
	if thread == 0 || !objc.Send[bool](objc.ID(thread), objc.RegisterName("isMainThread")) {
		return 0
	}
	app := objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	if app == 0 || !objc.Send[bool](app, objc.RegisterName("isActive")) {
		return 0
	}
	// currentInputContext may be nil between AppKit event dispatches. Use
	// the key window's existing first responder, whose context MyGo owns.
	responder := app.Send(objc.RegisterName("keyWindow")).Send(objc.RegisterName("firstResponder"))
	selector := objc.RegisterName("inputContext")
	if responder == 0 || !objc.Send[bool](responder, objc.RegisterName("respondsToSelector:"), selector) {
		return 0
	}
	return responder.Send(selector)
}

func inputSourceID(context objc.ID) string {
	source := context.Send(objc.RegisterName("selectedKeyboardInputSource"))
	if source == 0 {
		return ""
	}
	return objc.Send[string](source, objc.RegisterName("UTF8String"))
}

func setInputSource(context objc.ID, source string) error {
	name := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), source)
	available := context.Send(objc.RegisterName("keyboardInputSources"))
	if name == 0 || available == 0 || !objc.Send[bool](available, objc.RegisterName("containsObject:"), name) {
		return fmt.Errorf("%w: %s", ErrInputSourceUnavailable, source)
	}
	context.Send(objc.RegisterName("setSelectedKeyboardInputSource:"), name)
	if inputSourceID(context) != source {
		return fmt.Errorf("command: could not select input source %s", source)
	}
	return nil
}

func selectInputSource(source string) (func() error, error) {
	context := nativeInputContext()
	if context == 0 {
		return nil, ErrInputSourceUnavailable
	}
	previous := inputSourceID(context)
	if previous == source {
		return nil, nil
	}
	if err := setInputSource(context, source); err != nil {
		return nil, err
	}
	return func() error {
		// Query only the current live context; never message a saved object
		// that might have gone away with its window.
		current := nativeInputContext()
		if current == 0 || current != context || previous == "" || inputSourceID(current) != source {
			return nil
		}
		return setInputSource(current, previous)
	}, nil
}
