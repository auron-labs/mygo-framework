package components

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

type colorFormat int

const (
	colorRGBA colorFormat = iota
	colorHSBA
	colorHSLA
	colorAlpha // An opacity-only edit leaves both latent color models untouched.
)

type colorHSL struct{ h, s, l float64 }

func (x colorHSV) hsl() colorHSL {
	l := x.v * (1 - x.s/2)
	s := 0.0
	if l > 0 && l < 1 {
		s = (x.v - l) / min(l, 1-l)
	}
	return colorHSL{x.h, max(0, min(1, s)), l}
}

func (x colorHSL) hsv() colorHSV {
	v := x.l + x.s*min(x.l, 1-x.l)
	s := 0.0
	if v > 0 {
		s = 2 * (1 - x.l/v)
	}
	return colorHSV{x.h, max(0, min(1, s)), v}
}

func colorDecimal(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

func colorCopyText(c ui.Color, format string) string {
	c = c.SRGB()
	alpha := strconv.FormatFloat(math.Round(float64(c.A)*1000/255)/1000, 'f', -1, 64)
	switch format {
	case "RGBA":
		return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, alpha)
	case "HSBA":
		x := colorToHSV(c)
		return fmt.Sprintf("hsba(%s, %s%%, %s%%, %s)", colorDecimal(x.h), colorDecimal(x.s*100), colorDecimal(x.v*100), alpha)
	case "HSLA":
		x := colorToHSV(c).hsl()
		return fmt.Sprintf("hsla(%s, %s%%, %s%%, %s)", colorDecimal(x.h), colorDecimal(x.s*100), colorDecimal(x.l*100), alpha)
	default:
		if c.A != 255 {
			return fmt.Sprintf("#%s%02X", colorHex(c), c.A)
		}
		return "#" + colorHex(c)
	}
}

// refreshModels keeps unquantized channel values and latent hue/saturation at
// achromatic endpoints. Opacity and format changes must not round these away.
func (s *colorViewState) refreshModels(c ui.Color, source colorFormat) {
	switch source {
	case colorAlpha:
		return
	case colorHSLA:
		s.hsv = s.hsl.hsv()
		return
	case colorRGBA:
		x := colorToHSV(c.SRGB())
		if x.s == 0 {
			x.h = s.hsv.h
		}
		if x.v == 0 {
			x.s = s.hsv.s
		}
		s.hsv = x
	}
	x := s.hsv.hsl()
	if x.l == 0 || x.l == 1 {
		x.s = s.hsl.s
	}
	s.hsl = x
}

func (s *colorViewState) channel(i int, c ui.Color) (value, maximum float64) {
	if i == 4 {
		return float64(c.A) * 100 / 255, 100
	}
	if s.format == colorHSBA {
		return [3]float64{s.hsv.h, s.hsv.s * 100, s.hsv.v * 100}[i], [3]float64{360, 100, 100}[i]
	}
	if s.format == colorHSLA {
		return [3]float64{s.hsl.h, s.hsl.s * 100, s.hsl.l * 100}[i], [3]float64{360, 100, 100}[i]
	}
	return float64([3]uint8{c.R, c.G, c.B}[i]), 255
}

func (s *colorViewState) setChannel(i int, v float64, c ui.Color) (ui.Color, colorFormat) {
	if i == 4 {
		c.A = uint8(math.Round(v * 255 / 100))
		return c, colorAlpha
	}
	switch s.format {
	case colorHSBA:
		switch i {
		case 0:
			s.hsv.h = v
		case 1:
			s.hsv.s = v / 100
		case 2:
			s.hsv.v = v / 100
		}
		return s.hsv.color(c.A), colorHSBA
	case colorHSLA:
		switch i {
		case 0:
			s.hsl.h = v
		case 1:
			s.hsl.s = v / 100
		case 2:
			s.hsl.l = v / 100
		}
		return s.hsl.hsv().color(c.A), colorHSLA
	default:
		c = c.SRGB()
		switch i {
		case 0:
			c.R = uint8(v)
		case 1:
			c.G = uint8(v)
		case 2:
			c.B = uint8(v)
		}
		return c, colorRGBA
	}
}

// acceptDraft validates the entire candidate, including paste/undo, while
// allowing empty, decimal-point and incomplete hex drafts during editing.
func (s *colorViewState) acceptDraft(i int, draft string, c ui.Color) bool {
	if i == 3 {
		text := strings.TrimPrefix(draft, "#")
		if len(text) > 8 {
			return false
		}
		for _, r := range text {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
				return false
			}
		}
		return true
	}
	decimal := i == 4 || s.format != colorRGBA
	percent := i == 4 || (s.format != colorRGBA && i > 0)
	if percent {
		draft = strings.TrimSuffix(draft, "%")
	}
	dots := 0
	for _, r := range draft {
		if r == '.' && decimal {
			dots++
			if dots > 1 {
				return false
			}
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	if draft == "" || draft == "." {
		return true
	}
	n, err := strconv.ParseFloat(draft, 64)
	_, hi := s.channel(i, c)
	return err == nil && !math.IsNaN(n) && n >= 0 && n <= hi
}
