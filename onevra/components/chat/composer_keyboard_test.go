package chat

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestComposerKeyboardOptions(t *testing.T) {
	off := false
	editor := composerEditorOptions(ComposerOptions{
		Keyboard:    ui.KeyboardEmail,
		Capitalize:  ui.CapitalizeSentences,
		Autofill:    ui.AutofillEmail,
		Autocorrect: &off,
	})
	if !editor.Multiline {
		t.Error("composer editor lost multiline")
	}
	if editor.Keyboard != ui.KeyboardEmail || editor.Capitalize != ui.CapitalizeSentences || editor.Autofill != ui.AutofillEmail {
		t.Errorf("options not passed through: %+v", editor)
	}
	if editor.Autocorrect == nil || *editor.Autocorrect {
		t.Errorf("autocorrect not passed through: %+v", editor.Autocorrect)
	}
	if editor.ReturnKey != ui.ReturnSend {
		t.Errorf("default return key = %v, want Send", editor.ReturnKey)
	}
	if editor = composerEditorOptions(ComposerOptions{ReturnKey: ui.ReturnDone}); editor.ReturnKey != ui.ReturnDone {
		t.Errorf("explicit return key overridden: %v", editor.ReturnKey)
	}
}
