package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
)

// trayIconPNG draws the tray icon in code rather than carrying a binary
// resource. It is black on transparent so platforms can tint it as a template.
var trayIconPNG = sync.OnceValue(func() []byte {
	const size = 16
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	black := color.RGBA{A: 255}
	for y := range size {
		for x := range size {
			dx := float64(x) + 0.5 - size/2
			dy := float64(y) + 0.5 - size/2
			radiusSquared := dx*dx + dy*dy
			// A ring with a filled center stays legible at the tray's small size.
			if radiusSquared <= 42 && radiusSquared >= 20 || radiusSquared <= 6 {
				img.Set(x, y, black)
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		panic(err)
	}
	return encoded.Bytes()
})
