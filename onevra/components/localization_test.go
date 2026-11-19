package components

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCatalogFollowsWindowLanguage(t *testing.T) {
	day := time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC)
	count := 0.0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			Calendar(c, &day)
			NumberInput(c, &count, 0, 10, 1).Label("数量")
		})
	}, 500, 500)
	tt.SetLocale("zh-Hans")
	for _, want := range []string{"2026年10月", "今天", "一", "下个月", "增加数量"} {
		if _, ok := tt.Find(want); !ok && !tt.HasText(want) {
			t.Errorf("zh-Hans: missing %q in %q", want, tt.Texts())
		}
	}
	tt.SetLocale("en")
	if _, ok := tt.Find("Next month"); !ok || !tt.HasText("October 2026") {
		t.Fatalf("en: %q", tt.Texts())
	}
}
