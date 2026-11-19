package chat

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAttachmentPreview(t *testing.T) {
	img := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 20, 30)))
	removed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			if AttachmentPreview(c, AttachmentPreviewOptions{Label: "image.png", Image: img, RemoveLabel: "Remove image"}).Remove {
				removed++
			}
			AttachmentPreview(c, AttachmentPreviewOptions{Label: "read-only", Image: img})
			AttachmentPreview(c, AttachmentPreviewOptions{Label: "unavailable"})
		})
	}, 500, 200)
	if r, ok := tt.Find("image.png"); !ok || r.W != 62 || r.H != 62 {
		t.Fatalf("preview: %+v found=%v", r, ok)
	}
	if err := tt.Click("Remove image"); err != nil || removed != 1 {
		t.Fatalf("removal: %v count=%d", err, removed)
	}
	if !tt.HasText("read-only") || !tt.HasText("unavailable") {
		t.Fatal("read-only/placeholder labels missing")
	}
}

func TestAttachmentChip(t *testing.T) {
	removed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			if Attachment(c, AttachmentOptions{Label: "notes.md", Detail: "2 KB", RemoveLabel: "Remove notes.md"}).Remove {
				removed++
			}
			Attachment(c, AttachmentOptions{Label: "sent.txt"})
		})
	}, 500, 200)
	if !tt.HasText("notes.md") || !tt.HasText("2 KB") || !tt.HasText("sent.txt") {
		t.Fatal("attachment labels missing")
	}
	if err := tt.Click("Remove notes.md"); err != nil || removed != 1 {
		t.Fatalf("removal: %v count=%d", err, removed)
	}
}

func TestComposerPasteAndDisabledSend(t *testing.T) {
	for _, glass := range []bool{false, true} {
		draft := "keep"
		disabled, take, attachments := true, true, false
		pastes, sends := 0, 0
		tt := ui.NewTester(func(c *ui.Context) {
			options := ComposerOptions{
				Label: "Message", SendLabel: "Send", SendDisabled: disabled,
				OnPaste: func(string) bool { pastes++; return take },
			}
			if attachments {
				options.Attachments = func(c *ui.Context) { AttachmentPreview(c, AttachmentPreviewOptions{Label: "Image"}) }
			}
			var r ComposerResult
			if glass {
				r = GlassComposer(c, nil, &draft, options)
			} else {
				r = Composer(c, &draft, options)
			}
			if r.Send {
				sends++
			}
		}, 500, 300)
		if err := tt.Click("Message"); err != nil {
			t.Fatal(err)
		}
		tt.SetClipboard("file name")
		tt.Key(ui.Cmd, ui.KeyV)
		tt.Command("paste")
		if draft != "keep" || pastes != 2 {
			t.Fatalf("intercepted paste: %q calls=%d", draft, pastes)
		}
		tt.Key(0, ui.KeyEnter)
		_ = tt.Click("Send")
		if sends != 0 || draft != "keep" {
			t.Fatal("disabled send changed the draft")
		}
		// Introducing the attachment row must not change the editor identity.
		attachments, disabled, take = true, false, false
		tt.Frame()
		if err := tt.Click("Message"); err != nil {
			t.Fatal(err)
		}
		tt.Command("selectAll")
		tt.Command("paste")
		if draft != "file name" {
			t.Fatalf("text paste fallback: %q", draft)
		}
		tt.Key(ui.Shift, ui.KeyEnter)
		tt.Key(0, ui.KeyEnter)
		if sends != 1 || draft != "file name\n" {
			t.Fatalf("Enter regression: sends=%d draft=%q", sends, draft)
		}
	}
}
