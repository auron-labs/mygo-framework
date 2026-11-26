package imagecache

import (
	"bytes"
	"errors"
	"image"

	"github.com/egoist/mygo/ui"
)

const (
	// maxImageBytes bounds compressed input. Resty enforces it on network
	// reads, and readDiskFiles enforces it again because disk is untrusted.
	maxImageBytes = 64 << 20
	// maxImagePixels bounds the dimensions a raster decoder may expand before
	// MaxSide shrinking; decoding itself allocates the full raster once.
	maxImagePixels = 64 << 20
)

var (
	errImageEmpty         = errors.New("imagecache: image body is empty")
	errImageTooLarge      = errors.New("imagecache: image body exceeds the size limit")
	errImageTooManyPixels = errors.New("imagecache: image dimensions exceed the pixel limit")
	errImageDecode        = errors.New("imagecache: image cannot be decoded")
)

// decodeSafely decodes on a worker goroutine. Decoder panics and hostile
// dimensions are converted to errors; no image bytes or URLs are included in
// returned errors.
func decodeSafely(data []byte, maxSide int) (source ui.ImageSource, memoryBytes int64, err error) {
	defer func() {
		if recover() != nil {
			source, memoryBytes, err = nil, 0, errImageDecode
		}
	}()
	if len(data) == 0 {
		return nil, 0, errImageEmpty
	}
	if int64(len(data)) > maxImageBytes {
		return nil, 0, errImageTooLarge
	}

	if config, _, configErr := image.DecodeConfig(bytes.NewReader(data)); configErr == nil {
		pixels := int64(config.Width) * int64(config.Height)
		if pixels > maxImagePixels {
			return nil, 0, errImageTooManyPixels
		}
		bitmap, decodeErr := ui.DecodeBitmapScaled(data, maxSide)
		if decodeErr != nil {
			return nil, 0, errImageDecode
		}
		width, height := bitmap.Size()
		return bitmap, int64(width) * int64(height) * 4, nil
	}

	// SVG has no raster pixels to scale. Its compressed input size is already
	// bounded and is the retained memory charged to the LRU.
	source, decodeErr := ui.DecodeImage(data)
	if decodeErr != nil {
		return nil, 0, errImageDecode
	}
	return source, int64(len(data)), nil
}
