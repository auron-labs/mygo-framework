package imagecache

import (
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestImageBuildsPlaceholderThenInvalidatesWithDecodedImage(t *testing.T) {
	picture := testPNG(t, 16, 16, color.NRGBA{R: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(30 * time.Millisecond)
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	rawURL := server.URL + "/view.png"

	var avatarSource ui.ImageSource
	builds := 0
	tester := ui.NewTester(func(c *ui.Context) {
		builds++
		// Avatar-style custom rendering uses Load and owns the decoded source.
		avatarSource, _ = cache.Load(c, rawURL, LoadOptions{MaxSide: 16})
		cache.Image(c, rawURL, ImageOptions{
			MaxSide:     16,
			Placeholder: ui.RGB(0, 0, 255),
		}).Size(20, 20)
	}, 60, 60)

	initialBuilds := builds
	if avatarSource != nil {
		t.Fatalf("first frame already had an avatar source: %v", avatarSource)
	}
	initial := tester.Image().RGBAAt(10, 10)
	if initial.B < 200 || initial.R > 80 {
		t.Fatalf("first frame center = %+v, want blue placeholder", initial)
	}

	waitFor(t, "worker decode", func() bool { return hasDecoded(cache, rawURL, 16) })
	tester.Advance(20 * time.Millisecond)
	if builds <= initialBuilds {
		t.Fatalf("builds after worker invalidation = %d, want more than initial %d", builds, initialBuilds)
	}
	if avatarSource == nil {
		t.Fatal("avatar-style Load did not expose the decoded image after invalidation")
	}
	loaded := tester.Image().RGBAAt(10, 10)
	if loaded.R < 200 || loaded.B > 80 {
		t.Fatalf("loaded frame center = %+v, want red image", loaded)
	}
}

func TestImageKeepsPlaceholderAfterFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	rawURL := server.URL + "/broken-view.png"

	var loadErr error
	builds := 0
	tester := ui.NewTester(func(c *ui.Context) {
		builds++
		_, loadErr = cache.Load(c, rawURL, LoadOptions{MaxSide: 16})
		cache.Image(c, rawURL, ImageOptions{MaxSide: 16, Placeholder: ui.RGB(0, 0, 255)}).Size(20, 20)
	}, 60, 60)

	waitFor(t, "worker failure", func() bool { return cacheError(cache, rawURL, 16) != nil })
	tester.Advance(20 * time.Millisecond)
	if builds < 2 {
		t.Fatalf("builds after failure invalidation = %d, want at least 2", builds)
	}
	if loadErr == nil {
		t.Fatal("Load did not expose the negative-cached failure")
	}
	pixel := tester.Image().RGBAAt(10, 10)
	if pixel.B < 200 || pixel.R > 80 {
		t.Fatalf("failed frame center = %+v, want blue placeholder", pixel)
	}
}
