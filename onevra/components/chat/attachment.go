package chat

import (
	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// AttachmentPreviewOptions describes an application-prepared thumbnail.
// Label names the image for accessibility and its tooltip. An empty
// RemoveLabel makes it read-only. Loading and decoding belong to the host.
type AttachmentPreviewOptions struct {
	Label, RemoveLabel string
	Image              ui.ImageSource
}

// AttachmentPreviewResult reports a frame-local removal request.
type AttachmentPreviewResult struct {
	Root   ui.Element
	Remove bool
}

// AttachmentPreview shows a 64-point image tile with an optional remove
// action. A nil image shows a placeholder, without reading any file. Use it
// for rows of images only; Attachment keeps mixed rows at one height.
func AttachmentPreview(c *ui.Context, options AttachmentPreviewOptions) AttachmentPreviewResult {
	var result AttachmentPreviewResult
	t := c.Theme()
	result.Root = ui.Box(c).Size(64, 64).Shrink(0).Radius(8).Clip().
		Border(1, strongBorder(t)).Background(t.Surface).Children(func() {
		if options.Image != nil {
			ui.Image(c, options.Image).Label(options.Label).Size(62, 62).Fit(ui.Cover).Shrink(0)
		} else {
			ui.Box(c).Label(options.Label).Size(62, 62).Center().Children(func() {
				components.Icon(c, lucide.Image).Size(20, 20).TextColor(t.TextMuted)
			})
		}
		if options.RemoveLabel != "" {
			// Under a coarse pointer the hit square grows past the 18-point
			// badge it holds, which stays put in the tile's corner.
			hit := max(float32(18), components.TouchMin(c))
			b := ui.ButtonBase(c).Label(options.RemoveLabel).Absolute().Top(3).Right(3).
				Size(hit, hit).Radius(5).Justify(ui.End).AlignItems(ui.Start).
				Background(ui.Color{}).TextColor(t.Text)
			b.Children(func() {
				badge := ui.Box(c).Size(18, 18).Radius(5).Center().Background(t.Background.Alpha(.9))
				if b.Hovered() || b.Focused() {
					badge.Background(t.Surface).Border(1, t.Accent)
				}
				badge.Children(func() { components.Icon(c, lucide.X).Size(11, 11) })
			})
			result.Remove = b.Clicked()
		}
	})
	components.Tooltip(c, result.Root, options.Label)
	return result
}

// AttachmentOptions describes a file the host has already prepared. Detail
// is short trailing text, such as a size. Image is an optional decoded
// thumbnail; without one, Icon replaces the default file icon, such as
// lucide.Image for an image still being prepared. An empty RemoveLabel
// makes the chip read-only, as in a sent prompt.
type AttachmentOptions struct {
	Label, Detail, RemoveLabel string
	Image                      ui.ImageSource
	Icon                       func() *ui.SVG
}

// AttachmentResult reports a frame-local removal request.
type AttachmentResult struct {
	Root   ui.Element
	Remove bool
}

// Attachment shows a compact file chip with an optional remove action, for
// the composer's Attachments slot or a prompt. It performs no file I/O.
func Attachment(c *ui.Context, options AttachmentOptions) AttachmentResult {
	var result AttachmentResult
	t := c.Theme()
	// A coarse pointer gets the touch minimum's height and remove square; a
	// fine one the compact 30 and 18.
	result.Root = ui.Row(c).MinWidth(0).MaxWidth(220).Height(max(float32(30), components.TouchMin(c))).AlignItems(ui.Center).Gap(6).Padding(0, 4).
		Radius(7).Border(1, strongBorder(t)).Background(t.Background.Mix(t.Text, .035)).Children(func() {
		if options.Image != nil {
			ui.Image(c, options.Image).Size(22, 22).Fit(ui.Cover).Radius(4).Shrink(0)
		} else {
			icon := options.Icon
			if icon == nil {
				icon = lucide.FileText
			}
			ui.Row(c).Size(22, 22).Shrink(0).Center().Children(func() {
				components.Icon(c, icon).Size(13, 13).TextColor(tertiary(t))
			})
		}
		components.Text(c, options.Label).MinWidth(0).SingleLine().Ellipsis("…").FontSize(12.5).TextColor(secondary(t))
		if options.Detail != "" {
			components.Text(c, options.Detail).Shrink(0).FontSize(11.5).TextColor(tertiary(t))
		}
		if options.RemoveLabel == "" {
			ui.Box(c).Width(2)
			return
		}
		hit := max(float32(18), components.TouchMin(c))
		b := ui.ButtonBase(c).Label(options.RemoveLabel).Size(hit, hit).Shrink(0).Radius(4).Center().Background(ui.Color{}).TextColor(tertiary(t))
		if b.Hovered() || b.Focused() {
			b.Background(overlayStrong(t)).TextColor(t.Text)
		}
		b.Children(func() { components.Icon(c, lucide.X).Size(11, 11) })
		result.Remove = b.Clicked()
	})
	return result
}
