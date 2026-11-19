package mygomcp

import (
	"context"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	dragSteps          = 8
	defaultLongPressMS = 600
)

type longPressInput struct {
	clickRequest
	HoldMS *float64 `json:"hold_ms,omitempty" jsonschema:"Integer simulated time the finger rests before lifting, 0 through 1000; default 600"`
}

func (in longPressInput) validate() error {
	if err := in.clickRequest.validate(); err != nil {
		return err
	}
	return validateHold(in.HoldMS, defaultLongPressMS)
}

type dragInput struct {
	clickRequest
	ToX        float64  `json:"to_x" jsonschema:"Horizontal observation pixel coordinate where the finger ends"`
	ToY        float64  `json:"to_y" jsonschema:"Vertical observation pixel coordinate where the finger ends"`
	DurationMS float64  `json:"duration_ms" jsonschema:"Integer simulated time the finger takes to move, 0 through 1000; a drag lifted while moving flings, which needs about 1 through 800"`
	HoldMS     *float64 `json:"hold_ms,omitempty" jsonschema:"Integer simulated time the finger rests at the end before lifting, 0 through 1000; default 0, which lifts while still moving"`
	Cancel     bool     `json:"cancel,omitempty" jsonschema:"End with a system cancel, as when a system gesture or a call takes the finger, instead of lifting"`
}

func (in dragInput) validate() error {
	if err := in.clickRequest.validate(); err != nil {
		return err
	}
	if !finite(in.ToX) || !finite(in.ToY) || in.ToX < 0 || in.ToY < 0 {
		return &targetError{Code: "out_of_bounds", Message: "to_x and to_y must be finite nonnegative pixel coordinates"}
	}
	if err := (advanceInput{Milliseconds: in.DurationMS}).validate(); err != nil {
		return err
	}
	return validateHold(in.HoldMS, 0)
}

func validateHold(hold *float64, def float64) error {
	return (advanceInput{Milliseconds: holdMS(hold, def)}).validate()
}

func holdMS(hold *float64, def float64) float64 {
	if hold == nil {
		return def
	}
	return *hold
}

type viewportInput struct {
	SessionID string  `json:"session_id"`
	Width     int     `json:"width" jsonschema:"Window width in DIPs, including the areas the insets cover"`
	Height    int     `json:"height" jsonschema:"Window height in DIPs, including the areas the insets cover"`
	Scale     float32 `json:"scale,omitempty" jsonschema:"Device pixels per DIP, up to 4; omit to keep the current scale"`
	Top       float64 `json:"inset_top,omitempty" jsonschema:"DIPs covered at the top, as by a status bar or cutout"`
	Right     float64 `json:"inset_right,omitempty" jsonschema:"DIPs covered at the right"`
	Bottom    float64 `json:"inset_bottom,omitempty" jsonschema:"DIPs covered at the bottom, as by a home indicator, navigation bar or keyboard"`
	Left      float64 `json:"inset_left,omitempty" jsonschema:"DIPs covered at the left, as by a landscape cutout"`
}

func (in viewportInput) validate() error {
	for _, v := range []float64{in.Top, in.Right, in.Bottom, in.Left} {
		if !finite(v) || v < 0 || v > maxDimension {
			return errors.New("insets must be finite DIPs from 0 through 4096")
		}
	}
	if in.Top+in.Bottom >= float64(in.Height) || in.Left+in.Right >= float64(in.Width) {
		return errors.New("insets must leave room for content")
	}
	return nil
}

type viewportInfo struct {
	status
	ContentWidth  int     `json:"content_width"`
	ContentHeight int     `json:"content_height"`
	Scale         float32 `json:"scale"`
}

func (s *service) registerTouchTools(server *mcp.Server) {
	const note = " Delivered as a touch screen finger, not a mouse: it pans scrollable content under it and has no hover. Requires a fresh observation; set inspect=true for resulting targets without a PNG, or screenshot=true to include a PNG."
	addTool(server, "mygo_tap", "Touch a finger down and up at an observation image pixel point without advancing time."+note, func(ctx context.Context, in clickRequest) (payload, error) {
		return s.operation(ctx, in.SessionID, in.validate, false, func(x *session) (payload, error) {
			cx, cy, err := pointTarget(x, in)
			if err != nil {
				return payload{}, err
			}
			x.shot = nil
			x.tester.FingerDown(cx, cy)
			x.tester.FingerUp(cx, cy)
			return finishAction(x, in.Inspect, in.Screenshot)
		})
	})
	addTool(server, "mygo_long_press", "Hold a finger down at an observation image pixel point for hold_ms of simulated time (default 600), then lift."+note, func(ctx context.Context, in longPressInput) (payload, error) {
		return s.operation(ctx, in.SessionID, in.validate, false, func(x *session) (payload, error) {
			cx, cy, err := pointTarget(x, in.clickRequest)
			if err != nil {
				return payload{}, err
			}
			x.shot = nil
			x.tester.FingerDown(cx, cy)
			x.tester.Advance(time.Duration(holdMS(in.HoldMS, defaultLongPressMS)) * time.Millisecond)
			x.tester.FingerUp(cx, cy)
			return finishAction(x, in.Inspect, in.Screenshot)
		})
	})
	addTool(server, "mygo_drag", "Put a finger down at x,y, move it to to_x,to_y over duration_ms of simulated time, rest for hold_ms, then lift or cancel. A drag that lifts while moving (hold_ms 0, duration_ms about 1-800) flings the content on; advance time to see it settle."+note, func(ctx context.Context, in dragInput) (payload, error) {
		return s.operation(ctx, in.SessionID, in.validate, false, func(x *session) (payload, error) {
			cx, cy, err := pointTarget(x, in.clickRequest)
			if err != nil {
				return payload{}, err
			}
			tx, ty, err := imagePoint(x, in.ToX, in.ToY)
			if err != nil {
				return payload{}, err
			}
			x.shot = nil
			x.tester.FingerDown(cx, cy)
			step := time.Duration(in.DurationMS) * time.Millisecond / dragSteps
			for i := 1; i <= dragSteps; i++ {
				f := float32(i) / dragSteps
				x.tester.Advance(step)
				x.tester.FingerMove(cx+(tx-cx)*f, cy+(ty-cy)*f)
			}
			x.tester.Advance(time.Duration(holdMS(in.HoldMS, 0)) * time.Millisecond)
			if in.Cancel {
				x.tester.FingerCancel()
			} else {
				x.tester.FingerUp(tx, ty)
			}
			return finishAction(x, in.Inspect, in.Screenshot)
		})
	})
	addTool(server, "mygo_set_viewport", "Resize a fixture's window and set the DIPs the system bars, cutout and keyboard cover at each edge (safe-area insets); the view lays out inside them. Insets are set absolutely: omitted edges are 0. Invalidates the current observation; returns the new geometry, so inspect again.", func(ctx context.Context, in viewportInput) (payload, error) {
		return s.operation(ctx, in.SessionID, in.validate, true, func(x *session) (payload, error) {
			scale := in.Scale
			if scale == 0 {
				scale = x.fixture.Scale
			}
			pixels, err := fixturePixels(in.Width, in.Height, scale)
			if err != nil {
				return payload{}, err
			}
			total := pixels
			for _, other := range s.sessions {
				if other != x {
					p, _ := fixturePixels(other.fixture.Width, other.fixture.Height, other.fixture.Scale)
					total += p
				}
			}
			if total > maxTotalPixels {
				return payload{}, errors.New("total fixture pixel allocation exceeds limits")
			}
			x.fixture.Width, x.fixture.Height, x.fixture.Scale = in.Width, in.Height, scale
			x.tester.SetScale(scale)
			x.tester.SetSize(in.Width, in.Height)
			x.tester.SetInsets(float32(in.Top), float32(in.Right), float32(in.Bottom), float32(in.Left))
			return payload{metadata: viewportInfo{status: currentStatus(x, true), ContentWidth: in.Width, ContentHeight: in.Height, Scale: scale}}, nil
		})
	})
}
