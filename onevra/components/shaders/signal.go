package shaders

import "github.com/egoist/mygo/ui"

// SignalField fills its parent with a sparse grid of dots that breathe with
// slow noise, gather light around the pointer and pulse outward where it
// clicks. Its middle stays calmer than its edges, where content usually sits.
func SignalField(c *ui.Context, o Options) ui.Element { return signalField.build(c, o) }

var signalField = newEffect("onevra.shaders.signal", signalSource, func(t ui.Theme) ui.Color { return t.TextMuted })

const signalSource = `package main

var Time float
var Pointer vec3
var Ripples [6]vec4
var Tint vec4
var Accent vec4
var Intensity float
var Scale float
` + noiseSource + `
func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	spacing := 22 * Scale
	center := (floor(pos/spacing) + 0.5) * spacing

	// Two slow octaves of noise breathe across the field.
	t := Time
	n := noise(center/(260*Scale)+vec2(t*0.035, t*0.021))*0.65 + noise(center/(110*Scale)-vec2(t*0.05, -t*0.03))*0.35
	ambient := smoothstep(0.3, 0.85, n)

	// The middle, where content sits, stays calmer than the edges.
	edge := length((center - size*0.5) / (size * 0.5))
	calm := mix(0.45, 1.0, smoothstep(0.2, 1.1, edge))

	d := center - Pointer.xy
	glow := Pointer.z * exp(-dot(d, d)/(2*150*150))

	// Ripples light the dots on an expanding ring and push them outward.
	ring := 0.0
	push := vec2(0)
	for i := 0; i < 6; i++ {
		rp := Ripples[i]
		if rp.w > 0 {
			v := center - rp.xy
			dist := length(v)
			fade := 1 - rp.z/1.4
			k := (dist - rp.z*520) / 34
			band := exp(-k*k) * fade * fade * rp.w
			ring += band
			if dist > 0.001 {
				push += v / dist * band
			}
		}
	}
	ring = min(ring, 1.0)
	shift := push * spacing * 0.22
	if length(shift) > spacing*0.3 {
		shift = normalize(shift) * spacing * 0.3
	}

	radius := (0.45 + 0.35*ambient + 0.7*glow + 0.9*ring) * min(Scale, 1.6)
	cover := 1 - smoothstep(radius-0.6, radius+0.6, distance(pos, center+shift))
	a := clamp(Intensity*(0.05+0.10*ambient*calm+0.32*glow+0.4*ring), 0.0, 1.0) * cover * Tint.a
	col := mix(Tint.rgb, Accent.rgb, clamp(glow*0.9+ring, 0.0, 1.0))

	// A faint light under the pointer and ripples, between the dots.
	pd := pos - Pointer.xy
	h := Intensity * Accent.a * (0.035*Pointer.z*exp(-dot(pd, pd)/(2*190*190)) + 0.025*ring)
	return vec4(col*a, a) + vec4(Accent.rgb*h, h)*(1-a)
}
`

// noiseSource is value noise for the backgrounds.
const noiseSource = `
func hash(p vec2) float {
	p3 := fract(vec3(p.x, p.y, p.x) * 0.1031)
	p3 += dot(p3, p3.yzx+33.33)
	return fract((p3.x + p3.y) * p3.z)
}

func noise(p vec2) float {
	i := floor(p)
	f := fract(p)
	u := f * f * (3 - 2*f)
	a := hash(i)
	b := hash(i + vec2(1, 0))
	c := hash(i + vec2(0, 1))
	d := hash(i + vec2(1, 1))
	return mix(mix(a, b, u.x), mix(c, d, u.x), u.y)
}
`
