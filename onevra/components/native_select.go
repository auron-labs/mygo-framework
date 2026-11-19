package components

import "github.com/egoist/mygo/ui"

// NativeSelectOption associates an application value with localized menu text.
// Values should be unique. Disabled options remain visible but cannot be chosen.
type NativeSelectOption[T comparable] struct {
	Value    T
	Label    string
	Disabled bool
}

// NativeSelectParts exposes the styled trigger and the current build's change.
// Trigger.Changed is not used: native menu choices are reported by Changed.
type NativeSelectParts struct {
	Trigger ui.Element
	changed bool
}

// Changed reports a different value chosen in this build. Handle it immediately
// after NativeSelect, on the UI thread; applications own persistence and effects.
func (s *NativeSelectParts) Changed() bool { return s.changed }

// NativeSelect chooses a value through MyGo's operating-system menu. The trigger
// uses Onevra styling, not an OS select widget. It displays the selected option's
// label (empty if absent); callers may replace its Children and should supply an
// accessible Label. Enter, Space and Down open the focused trigger's menu.
// An empty option list disables the trigger. Keep option order stable while open.
func NativeSelect[T comparable](c *ui.Context, selected *T, options []NativeSelectOption[T]) *NativeSelectParts {
	b := ui.ButtonBase(c).Disabled(len(options) == 0)
	styleSelectTrigger(c, b)
	s := &NativeSelectParts{Trigger: b}
	b.Menu(func(m *ui.Menu) {
		for _, option := range options {
			if m.Item(option.Label).Checked(*selected == option.Value).Disabled(option.Disabled).Chosen() && *selected != option.Value {
				*selected = option.Value
				s.changed = true
			}
		}
	})
	label := ""
	for _, option := range options {
		if option.Value == *selected {
			label = option.Label
			break
		}
	}
	b.Label(label).Children(func() { Text(c, label).SingleLine() })
	return s
}
