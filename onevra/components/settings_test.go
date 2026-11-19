package components

import (
	"image/color"
	"reflect"
	"slices"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func settingsFixture() []SettingsCategory {
	return []SettingsCategory{
		{ID: "account", Title: "Account", Section: "Personal", Groups: []SettingsGroup{{ID: "security", Title: "Security", Items: []SettingsItem{{ID: "alerts", Title: "Alerts", Description: "New sign-in", Keywords: []string{"邮件"}}}}}},
		{ID: "appearance", Title: "Appearance", Groups: []SettingsGroup{{ID: "colors", Title: "Colors", Items: []SettingsItem{{ID: "theme", Title: "Theme", Description: "Choose a mode", Keywords: []string{"dark", "深色"}}}}}},
	}
}

func TestSearchSettings(t *testing.T) {
	catalog := settingsFixture()
	for _, tc := range []struct {
		query string
		want  []SettingsResult
	}{
		{"", []SettingsResult{{0, 0, 0}, {1, 0, 0}}},
		{" \t ", []SettingsResult{{0, 0, 0}, {1, 0, 0}}},
		{" DARK appearance ", []SettingsResult{{1, 0, 0}}},
		{"personal SECURITY sign-in", []SettingsResult{{0, 0, 0}}},
		{"深色", []SettingsResult{{1, 0, 0}}},
		{"邮件", []SettingsResult{{0, 0, 0}}},
		{"dark alerts", nil},
		{"missing", nil},
	} {
		if got := SearchSettings(catalog, tc.query); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %v, want %v", tc.query, got, tc.want)
		}
	}
	if got := SearchSettings(nil, ""); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestSettingsStateAndBuilders(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{Category: "obsolete"}
		catalog := settingsFixture()
		enabled := false
		catalog[1].Groups[0].Items[0].Build = func(c *ui.Context) { Switch(c, &enabled, "Enable theme") }
		tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, catalog, SettingsOptions{Narrow: narrow}) }, 800, 600)
		tt.Frame()
		if state.Category != "account" || !tt.HasText("Alerts") || tt.HasText("Theme") {
			t.Fatal("initial selection", tt.Texts())
		}
		state.Query = "dark"
		tt.Frame()
		if !tt.HasText("Theme") || tt.HasText("Alerts") || state.Category != "account" {
			t.Fatal("global search", tt.Texts())
		}
		if err := tt.Click("Enable theme"); err != nil {
			t.Fatal(err)
		}
		if !enabled {
			t.Fatal("result control did not update app state")
		}
		state.Query = ""
		tt.Frame()
		if !tt.HasText("Alerts") || tt.HasText("Theme") {
			t.Fatal("search did not restore category")
		}
		state.Query = "unmatched"
		tt.Frame()
		if !tt.HasText("No matching settings. Try another search.") {
			t.Fatal("empty result")
		}
		if narrow {
			if err := tt.Click("Settings category"); err != nil {
				t.Fatal(err)
			}
		}
		if err := tt.Click("Appearance"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if state.Query != "" || state.Category != "appearance" || !tt.HasText("Theme") {
			t.Fatal("category navigation", state, tt.Texts())
		}
		catalog = nil
		tt.Frame()
		if !tt.HasText("No matching settings. Try another search.") {
			t.Fatal("empty catalog")
		}
	}
}

func TestSettingsSidebarSpacing(t *testing.T) {
	catalog := []SettingsCategory{
		{ID: "account", Title: "Account", Section: "Personal"},
		{ID: "notifications", Title: "Notifications", Section: "Personal"},
		{ID: "general", Title: "General", Section: "Application"},
		{ID: "appearance", Title: "Appearance", Section: "Application"},
		{ID: "about", Title: "About"},
		{ID: "help", Title: "Help"},
		{ID: "privacy", Title: "Privacy", Section: "Personal"},
	}
	for _, mode := range []bool{false, true} {
		state := SettingsState{}
		var itemGap, sectionGap float32
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			itemGap, sectionGap = c.Theme().Space(1), c.Theme().Space(3)
			Settings(c, &state, catalog, SettingsOptions{})
		}, 800, 600)
		find := func(label string) ui.Rect {
			t.Helper()
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("missing navigation element %q", label)
			}
			return r
		}
		for _, pair := range [][2]string{{"Account", "Notifications"}, {"General", "Appearance"}, {"About", "Help"}} {
			first, second := find(pair[0]), find(pair[1])
			if gap := second.Y - first.Y - first.H; gap != itemGap {
				t.Fatalf("%v: item gap = %v, want %v", pair, gap, itemGap)
			}
		}
		notifications, application := find("Notifications"), find("Application")
		appearance, about := find("Appearance"), find("About")
		for _, pair := range [][2]ui.Rect{{notifications, application}, {appearance, about}} {
			if gap := pair[1].Y - pair[0].Y - pair[0].H; gap != sectionGap {
				t.Fatalf("section gap = %v, want %v", gap, sectionGap)
			}
		}
		// Repeated and unheaded sections must keep every category actionable.
		for _, category := range catalog {
			state.Query = "unmatched"
			tt.Frame()
			if err := tt.Click(category.Title); err != nil {
				t.Fatal(err)
			}
			if state.Category != category.ID || state.Query != "" {
				t.Fatalf("navigation to %q: %+v", category.ID, state)
			}
		}
	}
}

func TestSettingsLocalizedAndIndependent(t *testing.T) {
	a, b := SettingsState{Query: "not found"}, SettingsState{}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Key("a").Fill().Children(func() {
				Settings(c, &a, settingsFixture(), SettingsOptions{Narrow: true, Title: "偏好设置", ResultsTitle: "搜索结果", SearchLabel: "搜索设置", EmptyText: "无结果"})
			})
			ui.Column(c).Key("b").Fill().Children(func() { Settings(c, &b, settingsFixture(), SettingsOptions{Narrow: true}) })
		})
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("搜索结果") || !tt.HasText("无结果") || !tt.HasText("Alerts") || b.Query != "" {
		t.Fatal(tt.Texts())
	}
}

func TestSettingsGroupActions(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{}
		catalog := settingsFixture()
		clicks := 0
		catalog[0].Groups[0].Actions = func(c *ui.Context) {
			if Button(c, "Refresh", ButtonSecondary).Clicked() {
				clicks++
			}
		}
		tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, catalog, SettingsOptions{Narrow: narrow}) }, 800, 600)
		tt.Frame()
		title, ok := tt.Find("Security")
		action, found := tt.Find("Refresh")
		if !ok || !found || action.X <= title.X || action.Y > title.Y+title.H || action.Y+action.H < title.Y {
			t.Fatalf("narrow %v: action not trailing the group title: %+v %+v", narrow, title, action)
		}
		if err := tt.Click("Refresh"); err != nil || clicks != 1 {
			t.Fatalf("narrow %v: action not clicked: %v %d", narrow, err, clicks)
		}
		// Actions follow their group into search results but are not searched.
		state.Query = "Refresh"
		tt.Frame()
		if tt.HasText("Refresh") || tt.HasText("Alerts") {
			t.Fatal("actions were searched", tt.Texts())
		}
		state.Query = "sign-in"
		tt.Frame()
		if !tt.HasText("Refresh") {
			t.Fatal("actions missing from search results", tt.Texts())
		}
		state.Query = ""
		state.Category = "appearance"
		tt.Frame()
		if tt.HasText("Refresh") {
			t.Fatal("actions shown outside their category", tt.Texts())
		}
	}
}

func TestSettingsSectionActions(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{}
		var sections []string
		clicks := 0
		opts := SettingsOptions{Narrow: narrow, SectionActions: func(c *ui.Context, section string) {
			sections = append(sections, section)
			if IconButton(c, lucide.Plus(), "Add "+section, IconButtonCompact).Clicked() {
				clicks++
			}
		}}
		tt := ui.NewTester(func(c *ui.Context) {
			sections = nil
			Settings(c, &state, settingsFixture(), opts)
		}, 800, 600)
		tt.Frame()
		// Only the named section has actions; in the narrow layout, only the
		// selected category's.
		if !slices.Equal(sections, []string{"Personal"}) {
			t.Fatalf("narrow %v: actions built for %v", narrow, sections)
		}
		add, ok := tt.Find("Add Personal")
		anchor := "Personal"
		if narrow {
			anchor = "Account"
		}
		heading, found := tt.Find(anchor)
		if !ok || !found || add.X <= heading.X || add.Y > heading.Y+heading.H || add.Y+add.H < heading.Y {
			t.Fatalf("narrow %v: action not trailing %q: %+v %+v", narrow, anchor, heading, add)
		}
		if err := tt.Click("Add Personal"); err != nil || clicks != 1 {
			t.Fatalf("narrow %v: action not clicked: %v %d", narrow, err, clicks)
		}
		if narrow {
			state.Category = "appearance"
			tt.Frame()
			if len(sections) != 0 || tt.HasText("Add Personal") {
				t.Fatal("actions shown for a category without a section", sections)
			}
		}
	}
}

func TestSettingsPlaceholder(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{Category: "services"}
		catalog := append(settingsFixture(), SettingsCategory{
			ID: "services", Title: "Services", Section: "Connected",
			Placeholder: func(c *ui.Context) { Text(c, "None yet") },
			Groups:      []SettingsGroup{{ID: "hidden", Items: []SettingsItem{{ID: "hidden", Title: "Hidden"}}}},
		})
		tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, catalog, SettingsOptions{Narrow: narrow}) }, 800, 600)
		tt.Frame()
		// A placeholder is never selected, so Settings opens on the first page.
		if state.Category != "account" || !tt.HasText("Security") || tt.HasText("Hidden") {
			t.Fatalf("narrow %v: placeholder selected: %q %v", narrow, state.Category, tt.Texts())
		}
		if narrow {
			if tt.HasText("None yet") {
				t.Fatal("placeholder shown without a sidebar")
			}
			continue
		}
		heading, ok := tt.Find("Connected")
		note, found := tt.Find("None yet")
		if !ok || !found || note.Y <= heading.Y {
			t.Fatalf("placeholder not under its section: %+v %+v", heading, note)
		}
		if err := tt.Click("None yet"); err != nil || state.Category != "account" {
			t.Fatalf("placeholder navigated: %v %q", err, state.Category)
		}
		state.Query = "Hidden"
		tt.Frame()
		if tt.HasText("Hidden") {
			t.Fatal("placeholder searched", tt.Texts())
		}
	}
}

func TestSettingsDescriptionContent(t *testing.T) {
	state := SettingsState{}
	catalog := settingsFixture()
	clicks := 0
	catalog[0].Groups[0].Items[0].DescriptionContent = func(c *ui.Context) {
		ui.Text(c, "New sign-in · ")
		if Button(c, "3 devices", ButtonLink).Clicked() {
			clicks++
		}
	}
	tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, catalog, SettingsOptions{}) }, 800, 600)
	tt.Frame()
	title, ok := tt.Find("Alerts")
	link, found := tt.Find("3 devices")
	if !ok || !found || link.Y <= title.Y || !slices.Contains(tt.Texts(), "New sign-in · 3 devices") || slices.Contains(tt.Texts(), "New sign-in") {
		t.Fatalf("content not below the title in place of the description: %+v %+v %v", title, link, tt.Texts())
	}
	if err := tt.Click("3 devices"); err != nil || clicks != 1 {
		t.Fatalf("link not clicked: %v %d", err, clicks)
	}
	// Search still matches Description and shows the content.
	state.Query = "sign-in"
	tt.Frame()
	if !tt.HasText("3 devices") {
		t.Fatal("content missing from search results", tt.Texts())
	}
}

func TestSettingsCategoryImage(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	catalog := []SettingsCategory{
		{ID: "general", Title: "General", Icon: lucide.Settings()},
		{ID: "project", Title: "Project", Icon: lucide.Folder(), Image: solidBitmap(16, 16, red)},
	}
	state := SettingsState{}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Settings(c, &state, catalog, SettingsOptions{})
	}, 800, 600)
	general, _ := tt.Find("General")
	project, ok := tt.Find("Project")
	if !ok || project.H != general.H {
		t.Fatalf("project category = %v, want the height of %v", project, general)
	}
	found := false
	for x := project.X; x < project.X+project.H && !found; x++ {
		found = tt.Image().RGBAAt(int(x), int(project.Y+project.H/2)) == red
	}
	if !found {
		t.Fatal("category image replaced by its icon")
	}
	if err := tt.Click("Project"); err != nil || state.Category != "project" {
		t.Fatalf("navigation to the image category: %v %+v", err, state)
	}
}

func TestSettingsHideSearchAndSidebar(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{Query: "dark"}
		opts := SettingsOptions{Narrow: narrow, HideSearch: true}
		tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, settingsFixture(), opts) }, 800, 600)
		tt.Frame()
		if _, ok := tt.Find("Search settings"); ok || !tt.HasText("Alerts") || tt.HasText("Theme") || narrow == tt.HasText("Appearance") {
			t.Fatalf("narrow %v: hidden search: %v", narrow, tt.Texts())
		}
		// Without navigation every category shares one page under its title.
		opts = SettingsOptions{Narrow: narrow, HideSidebar: true}
		state.Query = ""
		tt.Frame()
		if _, ok := tt.Find("Settings category"); ok || !tt.HasText("Alerts") || !tt.HasText("Theme") || !tt.HasText("Account") || !tt.HasText("Appearance") {
			t.Fatalf("narrow %v: hidden sidebar: %v", narrow, tt.Texts())
		}
		alerts, _ := tt.Find("Alerts")
		if field, ok := tt.Find("Search settings"); !ok || field.Y >= alerts.Y {
			t.Fatalf("narrow %v: search not above the rows: %v", narrow, tt.Texts())
		}
		state.Query = "dark"
		tt.Frame()
		if tt.HasText("Alerts") || !tt.HasText("Theme") || !tt.HasText("Search results") {
			t.Fatalf("narrow %v: search without sidebar: %v", narrow, tt.Texts())
		}
		// A single category drops its redundant title.
		state.Query = ""
		single := settingsFixture()[:1]
		tt = ui.NewTester(func(c *ui.Context) { Settings(c, &state, single, opts) }, 800, 600)
		tt.Frame()
		if tt.HasText("Account") || !tt.HasText("Alerts") || !tt.HasText("Settings") {
			t.Fatalf("narrow %v: single category: %v", narrow, tt.Texts())
		}
	}
}

func TestSettingsItemSlots(t *testing.T) {
	for _, narrow := range []bool{false, true} {
		state := SettingsState{}
		catalog := settingsFixture()
		name, clicks := "", 0
		items := &catalog[0].Groups[0].Items
		(*items)[0].Leading = func(c *ui.Context) { Text(c, "Mark") }
		*items = append(*items,
			SettingsItem{ID: "name", Title: "Display name", Stacked: true, Build: func(c *ui.Context) {
				TextBox(c, &name, TextBoxOptions{}).Label("Name field").FillWidth()
			}},
			SettingsItem{ID: "profile", Title: "Profile card", Keywords: []string{"avatar"}, Content: func(c *ui.Context) {
				if Button(c, "Edit profile", ButtonSecondary).Clicked() {
					clicks++
				}
			}},
		)
		tt := ui.NewTester(func(c *ui.Context) { Settings(c, &state, catalog, SettingsOptions{Narrow: narrow}) }, 800, 600)
		tt.Frame()
		mark, _ := tt.Find("Mark")
		alerts, ok := tt.Find("Alerts")
		if !ok || mark.X+mark.W > alerts.X || mark.Y > alerts.Y+alerts.H {
			t.Fatalf("narrow %v: leading not before the title: %+v %+v", narrow, mark, alerts)
		}
		title, _ := tt.Find("Display name")
		field, ok := tt.Find("Name field")
		if !ok || field.Y < title.Y+title.H || field.X > title.X {
			t.Fatalf("narrow %v: stacked control not below its title: %+v %+v", narrow, title, field)
		}
		// Content replaces the displayed title but keeps it as search metadata.
		if tt.HasText("Profile card") {
			t.Fatal("content row showed its title", tt.Texts())
		}
		if err := tt.Click("Edit profile"); err != nil || clicks != 1 {
			t.Fatalf("content not interactive: %v %d", err, clicks)
		}
		state.Query = "avatar"
		tt.Frame()
		if !tt.HasText("Edit profile") || tt.HasText("Alerts") {
			t.Fatal("content row not searchable", tt.Texts())
		}
	}
}

func TestSettingsCloseButton(t *testing.T) {
	state := SettingsState{}
	closed := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Settings(c, &state, settingsFixture(), SettingsOptions{Close: func() { closed++ }})
	}, 800, 600)
	tt.Frame()
	if err := tt.Click("Close settings"); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("close button calls = %d", closed)
	}
	// The button is opt-in and its label is overridable.
	tt = ui.NewTester(func(c *ui.Context) { Settings(c, &state, settingsFixture(), SettingsOptions{}) }, 800, 600)
	tt.Frame()
	if _, ok := tt.Find("Close settings"); ok {
		t.Fatal("close button shown without a callback")
	}
	tt = ui.NewTester(func(c *ui.Context) {
		Settings(c, &state, settingsFixture(), SettingsOptions{Close: func() {}, CloseLabel: "Dismiss"})
	}, 800, 600)
	tt.Frame()
	if _, ok := tt.Find("Dismiss"); !ok {
		t.Fatal("close label override missing")
	}
}
