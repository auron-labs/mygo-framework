package components

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestAvatarInitials(t *testing.T) {
	for name, want := range map[string]string{
		"Maya Chen": "MC", "  leo  park  ": "LP", "Sara Ruiz Ortega": "SR", "ines": "I", "陈美雅": "陈", "": "",
	} {
		if got := avatarInitials(name); got != want {
			t.Errorf("%q: got %q, want %q", name, got, want)
		}
	}
}

func TestAvatarContentAndLabels(t *testing.T) {
	photo := ui.NewBitmap(image.NewNRGBA(image.Rect(0, 0, 8, 8)))
	var missing *ui.Bitmap
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Palette(false))
		ui.Row(c).Gap(8).Children(func() {
			Avatar(c, "Maya Chen", AvatarOptions{Status: AvatarOnline, Size: 88})
			Avatar(c, "Leo Park", AvatarOptions{Image: photo, Status: AvatarOffline})
			Avatar(c, "Sara Ruiz", AvatarOptions{Image: missing})
		})
	}, 300, 120)
	for _, label := range []string{"Maya Chen, online", "Leo Park, offline", "Sara Ruiz"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("missing %q in %v", label, tt.Texts())
		}
	}
	if r, _ := tt.Find("Maya Chen, online"); r.W != 88 || r.H != 88 {
		t.Fatalf("explicit size not applied: %+v", r)
	}
	if !tt.HasText("MC") || !tt.HasText("SR") || tt.HasText("LP") {
		t.Fatalf("initials should replace only missing photos: %v", tt.Texts())
	}
	tt.SetLocale("zh-Hans")
	if _, ok := tt.Find("Maya Chen，在线"); !ok {
		t.Fatal("status label not localized")
	}
}

func avatarTeam(names ...string) []AvatarGroupMember {
	members := make([]AvatarGroupMember, len(names))
	for i, name := range names {
		members[i] = AvatarGroupMember{Name: name}
	}
	return members
}

func TestAvatarGroupOverflow(t *testing.T) {
	members := avatarTeam("Maya Chen", "Leo Park", "Sara Ruiz", "Tom Hale", "Ines Ma")
	options := AvatarGroupOptions{Max: 3, Label: "Editors"}
	tt := ui.NewTester(func(c *ui.Context) {
		AvatarGroup(c, members, options)
	}, 400, 120)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	for _, label := range []string{"Editors", "Maya Chen", "Sara Ruiz", "2 more members", "+2"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("missing %q in %v", label, tt.Texts())
		}
	}
	if _, ok := tt.Find("Tom Hale"); ok {
		t.Fatal("overflowing member was built")
	}
	// Four slots overlapping by a sixth of the diameter, after an equal inset.
	if r, _ := tt.Find("Editors"); r.H < 30 || r.W != r.H/6+4*(r.H-r.H/6) {
		t.Fatalf("stack bounds %+v", r)
	}

	members = members[:4]
	tt.Frame()
	if !tt.HasText("+1") || tt.HasText("+2") {
		t.Fatalf("overflow count not updated: %v", tt.Texts())
	}
	options.Max = 0
	tt.Frame()
	if _, ok := tt.Find("Tom Hale"); !ok || tt.HasText("+1") {
		t.Fatalf("default maximum should show four members: %v", tt.Texts())
	}
	options = AvatarGroupOptions{Max: -1}
	tt.SetLocale("zh-Hans")
	if _, ok := tt.Find("团队成员"); !ok || !tt.HasText("+4") {
		t.Fatalf("negative maximum or localized label: %v", tt.Texts())
	}
	if _, ok := tt.Find("另外 4 位成员"); !ok {
		t.Fatal("overflow label not localized")
	}
}

func TestAvatarGroupHover(t *testing.T) {
	members := avatarTeam("Maya Chen", "Leo Park", "Sara Ruiz")
	for _, reverse := range []bool{false, true} {
		name := "front last"
		if reverse {
			name = "front first"
		}
		t.Run(name, func(t *testing.T) {
			positions := func(tt *ui.Tester) map[string]ui.Rect {
				boxes := map[string]ui.Rect{}
				for _, member := range members {
					r, ok := tt.Find(member.Name)
					if !ok {
						t.Fatalf("missing %q in %v", member.Name, tt.Texts())
					}
					boxes[member.Name] = r
				}
				return boxes
			}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Padding(60).Children(func() {
					AvatarGroup(c, members, AvatarGroupOptions{Reverse: reverse})
				})
			}, 300, 200)
			before := positions(tt)
			r := before["Leo Park"]
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(650 * time.Millisecond)
			tt.Frame()
			if !tt.HasText("Leo Park") {
				t.Fatal("hovered member did not name itself")
			}
			after := positions(tt)
			if after["Leo Park"].Y >= r.Y {
				t.Fatalf("hovered member did not rise: %+v from %+v", after["Leo Park"], r)
			}
			if after["Maya Chen"].X >= before["Maya Chen"].X {
				t.Fatalf("preceding member did not ease aside: %+v", after["Maya Chen"])
			}
			if after["Sara Ruiz"].X <= before["Sara Ruiz"].X {
				t.Fatalf("following member did not ease aside: %+v", after["Sara Ruiz"])
			}
		})
	}
}

// solidPhoto returns a one-color bitmap, so a test can tell avatars apart.
func solidPhoto(r, g, b uint8) *ui.Bitmap {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.NRGBA{r, g, b, 255})
		}
	}
	return ui.NewBitmap(img)
}

func TestAvatarGroupOverlap(t *testing.T) {
	members := avatarTeam("Maya Chen", "Leo Park", "Sara Ruiz")
	build := func(options AvatarGroupOptions) *ui.Tester {
		options.Label = "Editors"
		options.Size = 48
		tt := ui.NewTester(func(c *ui.Context) {
			AvatarGroup(c, members, options)
		}, 500, 120)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		return tt
	}
	distance := func(tt *ui.Tester) (group, gap float32) {
		r, ok := tt.Find("Editors")
		if !ok {
			t.Fatalf("missing group in %v", tt.Texts())
		}
		first, ok := tt.Find("Maya Chen")
		if !ok {
			t.Fatalf("missing first member in %v", tt.Texts())
		}
		second, ok := tt.Find("Leo Park")
		if !ok {
			t.Fatalf("missing second member in %v", tt.Texts())
		}
		return r.W, second.X - first.X
	}
	// A sixth of the diameter overlaps by default.
	group, gap := distance(build(AvatarGroupOptions{}))
	if gap != 40 || group != 48/6+3*(48-48/6) {
		t.Fatalf("default overlap: gap %v, group %v", gap, group)
	}
	// A larger overlap packs the avatars closer.
	group, gap = distance(build(AvatarGroupOptions{Overlap: 24}))
	if gap != 24 || group != 24+3*(48-24) {
		t.Fatalf("overlap 24: gap %v, group %v", gap, group)
	}
	// A negative overlap leaves room between them instead.
	group, gap = distance(build(AvatarGroupOptions{Overlap: -12}))
	if gap != 60 || group != 3*48+2*12 {
		t.Fatalf("overlap -12: gap %v, group %v", gap, group)
	}
}

func TestAvatarGroupReverse(t *testing.T) {
	red, blue := solidPhoto(200, 40, 40), solidPhoto(40, 80, 200)
	members := []AvatarGroupMember{
		{Name: "Maya Chen", Image: red},
		{Name: "Leo Park", Image: blue},
	}
	build := func(reverse bool, direction ui.Direction) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(false))
			ui.Box(c).Background(c.Theme().Background).Children(func() {
				AvatarGroup(c, members, AvatarGroupOptions{
					Size: 48, Overlap: 32, Reverse: reverse, Label: "Editors",
				})
			})
		}, 200, 100)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.SetDirection(direction)
		return tt
	}
	for _, direction := range []ui.Direction{ui.LeftToRight, ui.RightToLeft} {
		// Both stacks keep the members' order, so a point in their overlap
		// shows the avatar in front: the later one, or the first reversed.
		normal, reversed := build(false, direction), build(true, direction)
		first, _ := normal.Find("Maya Chen")
		second, _ := normal.Find("Leo Park")
		if (second.X > first.X) != (direction == ui.LeftToRight) {
			t.Fatalf("%v: members not laid out in reading order: %+v and %+v", direction, first, second)
		}
		nr, _ := normal.Find("Editors")
		rr, _ := reversed.Find("Editors")
		if nr != rr {
			t.Fatalf("%v: reverse moved the stack: %+v from %+v", direction, rr, nr)
		}
		x := second.X + 32/2
		if second.X < first.X {
			x = second.X + second.W - 32/2
		}
		px, py := int(x), int(first.Y+first.H/2)
		if got := normal.Image().RGBAAt(px, py); got.B < 150 || got.R > 100 {
			t.Fatalf("%v: later member should be in front: %v", direction, got)
		}
		if got := reversed.Image().RGBAAt(px, py); got.R < 150 || got.B > 100 {
			t.Fatalf("%v: first member should be in front: %v", direction, got)
		}
	}
}

func TestAvatarGroupHoverRing(t *testing.T) {
	members := avatarTeam("Maya Chen", "Leo Park", "Sara Ruiz")
	palette := theme.Palette(false)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(palette)
		ui.Column(c).Padding(60).Children(func() {
			AvatarGroup(c, members, AvatarGroupOptions{Size: 48})
		})
	}, 400, 200)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	accent := color.RGBA{palette.Accent.R, palette.Accent.G, palette.Accent.B, 255}
	hasAccent := func(img *image.RGBA, r ui.Rect) bool {
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if img.RGBAAt(x, y) == accent {
					return true
				}
			}
		}
		return false
	}
	r, _ := tt.Find("Leo Park")
	if hasAccent(tt.Image(), r) {
		t.Fatalf("member has an accent separator before the pointer arrives: %+v", r)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !hasAccent(tt.Image(), r) {
		t.Fatalf("hovered member has no accent separator: %+v", r)
	}
}
