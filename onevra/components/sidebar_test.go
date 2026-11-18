package components

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func sidebarFixture(dark bool, selected *int, disabled bool) func(*ui.Context) {
	return func(c *ui.Context) {
		c.SetTheme(theme.Palette(dark))
		Sidebar(c).Disabled(disabled).Children(func() {
			SidebarSection(c, "Locations", func() {
				if SidebarItem(c, "Home", lucide.House(), *selected == 0).Clicked() {
					*selected = 0
				}
				if SidebarItem(c, "Files", nil, *selected == 1).Clicked() {
					*selected = 1
				}
			})
			SidebarSection(c, "Account", nil)
		})
	}
}

func TestSidebarAppearance(t *testing.T) {
	for _, dark := range []bool{false, true} {
		selected := 0
		tt := ui.NewTester(sidebarFixture(dark, &selected, false), 220, 300)
		home, _ := tt.Find("Home")
		files, _ := tt.Find("Files")
		heading, _ := tt.Find("Locations")
		account, _ := tt.Find("Account")
		if heading.Y != 0 || home.X != 12 || home.W != 196 || files.Y-home.Y-home.H != 4 || account.Y-files.Y-files.H != 12 {
			t.Fatalf("unexpected sidebar spacing: %v %v %v %v", heading, home, files, account)
		}
		blank := ui.NewTester(func(c *ui.Context) { c.SetTheme(theme.Palette(dark)) }, 220, 300)
		if tt.Image().RGBAAt(20, 299) != blank.Image().RGBAAt(20, 299) {
			t.Fatal("sidebar must preserve its host background")
		}
		for _, r := range []ui.Rect{home, files} {
			tt.Move(219, 299)
			before := bytes.Clone(tt.Image().Pix)
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatal("hover changed sidebar appearance")
			}
		}
		if err := tt.Click("Files"); err != nil {
			t.Fatal(err)
		}
		if selected != 1 {
			t.Fatal("click did not select Files")
		}
		pixel := tt.Image().RGBAAt(int(files.X+files.W-12), int(files.Y+files.H/2))
		palette := theme.LightPalette()
		if dark {
			palette = theme.DarkPalette()
		}
		want := palette.Text.Alpha(.1).Over(palette.Background)
		delta := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
		if pixel.A != 255 || delta(pixel.R, want.R) > 1 || delta(pixel.G, want.G) > 1 || delta(pixel.B, want.B) > 1 {
			t.Fatalf("selection tint = %v, want %v", pixel, want)
		}
	}
}

func TestSidebarKeyboardAndDisabled(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		selected := -1
		tt := ui.NewTester(sidebarFixture(false, &selected, disabled), 220, 300)
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeyEnter)
		if !disabled && selected != 0 {
			t.Fatal("Enter did not activate first item")
		}
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeySpace)
		if !disabled && selected != 1 {
			t.Fatal("Space did not activate second item")
		}
		if err := tt.Click("Files"); err != nil {
			t.Fatal(err)
		}
		if disabled && selected != -1 {
			t.Fatal("disabled sidebar accepted input")
		}
	}
}

// solidBitmap is a w×h image of one color.
func solidBitmap(w, h int, color color.RGBA) *ui.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = color.R, color.G, color.B, color.A
	}
	return ui.NewBitmap(img)
}

func TestSidebarImageItem(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	// A wide image must fit the icon square whole: full width, letterboxed.
	wide := solidBitmap(80, 40, red)
	var size, inset float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		size, inset = c.Theme().Space(4.5), c.Theme().Space(2)
		Sidebar(c).Children(func() {
			SidebarItem(c, "Icon", lucide.House(), false)
			SidebarImageItem(c, "Image", wide, false)
			SidebarImageItem(c, "Empty", nil, false)
		})
	}, 220, 200)
	icon, _ := tt.Find("Icon")
	item, ok := tt.Find("Image")
	if !ok || item.H != icon.H {
		t.Fatalf("image item = %v, want the height of %v", item, icon)
	}
	x, y := item.X+inset, item.Y+item.H/2
	isRed := func(x, y float32) bool { return tt.Image().RGBAAt(int(x), int(y)) == red }
	if !isRed(x+size*.25, y) || !isRed(x+size*.75, y) {
		t.Fatal("image does not fill the icon square's width")
	}
	if isRed(x+size/2, y-size*.4) || isRed(x+size/2, y+size*.4) {
		t.Fatal("wide image was cropped or stretched instead of fitted")
	}
	if _, ok := tt.Find("Empty"); !ok {
		t.Fatal("missing item without an image")
	}
}

func TestSidebarItemWithSlots(t *testing.T) {
	label := "A project name long enough to need an ellipsis"
	var inset float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		inset = c.Theme().Space(2)
		Sidebar(c).Children(func() {
			SidebarItemWithSlots(c, label, false, nil, func(c *ui.Context, size float32) {
				ui.Box(c).Size(size, size).Shrink(0).Label("mark")
			})
		})
	}, 220, 200)
	item, ok := tt.Find(label)
	mark, found := tt.Find("mark")
	if !ok || !found {
		t.Fatal("missing item or its trailing mark")
	}
	// The label gives way so the mark stays inside the item at its end.
	if mark.X+mark.W > item.X+item.W || item.X+item.W-(mark.X+mark.W) > inset+1 {
		t.Fatalf("mark %v outside the end of item %v", mark, item)
	}
}

func TestSidebarItemWithContent(t *testing.T) {
	label := "Project"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Sidebar(c).Children(func() {
			SidebarItemWithContent(c, label, false, nil, func(c *ui.Context) ui.Element {
				return ui.Box(c).Size(400, 12).Shrink(0).Label("content")
			}, func(c *ui.Context, size float32) {
				ui.Box(c).Size(size, size).Shrink(0).Label("mark")
			})
		})
	}, 220, 200)
	item, ok := tt.Find(label)
	content, shown := tt.Find("content")
	mark, found := tt.Find("mark")
	if !ok || !shown || !found {
		t.Fatal("missing item, its content or its trailing mark")
	}
	// Wide content gives way like the label, keeping the mark inside the item.
	if content.X+content.W > mark.X || mark.X+mark.W > item.X+item.W {
		t.Fatalf("content %v and mark %v overflow item %v", content, mark, item)
	}
}

func TestSidebarSectionWithActions(t *testing.T) {
	clicks := 0
	build := func(actions bool) *ui.Tester {
		return ui.NewTester(func(c *ui.Context) {
			Sidebar(c).Children(func() {
				var add func(c *ui.Context)
				if actions {
					add = func(c *ui.Context) {
						if IconButton(c, lucide.Plus(), "Add", IconButtonCompact).Clicked() {
							clicks++
						}
					}
				}
				SidebarSectionWithActions(c, "Devices", add, func() { SidebarItem(c, "Laptop", nil, false) })
			})
		}, 220, 300)
	}
	plain, tt := build(false), build(true)
	before, _ := plain.Find("Laptop")
	heading, _ := tt.Find("Devices")
	item, _ := tt.Find("Laptop")
	add, ok := tt.Find("Add")
	if !ok || add.W != 20 || add.H != 20 {
		t.Fatalf("missing compact action: %+v", add)
	}
	// The action ends the heading's row without raising the heading.
	if item.Y != before.Y || add.X+add.W != 208 || add.Y < 0 || add.Y+add.H > item.Y {
		t.Fatalf("action misplaced: heading %+v action %+v item %+v, item was %+v", heading, add, item, before)
	}
	if err := tt.Click("Add"); err != nil || clicks != 1 {
		t.Fatalf("action not clicked: %v %d", err, clicks)
	}
}
