package imagecache

import (
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
	waitFor(t, "worker invalidation build", func() bool {
		tester.Advance(20 * time.Millisecond)
		return builds > initialBuilds
	})
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
	waitFor(t, "failure invalidation build", func() bool {
		tester.Advance(20 * time.Millisecond)
		return builds >= 2
	})
	if loadErr == nil {
		t.Fatal("Load did not expose the negative-cached failure")
	}
	pixel := tester.Image().RGBAAt(10, 10)
	if pixel.B < 200 || pixel.R > 80 {
		t.Fatalf("failed frame center = %+v, want blue placeholder", pixel)
	}
}

func TestNegativeCachedViewRecoversWhenRetryExpires(t *testing.T) {
	picture := testPNG(t, 16, 16, color.NRGBA{R: 255, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	cache.retryAfter = 150 * time.Millisecond
	rawURL := server.URL + "/retry-view.png"

	var avatarSource ui.ImageSource
	var loadErr error
	builds := 0
	tester := ui.NewTester(func(c *ui.Context) {
		builds++
		avatarSource, loadErr = cache.Load(c, rawURL, LoadOptions{MaxSide: 16})
	}, 60, 60)

	waitFor(t, "initial failure", func() bool { return cacheError(cache, rawURL, 16) != nil })
	waitFor(t, "negative failure build", func() bool {
		tester.Advance(10 * time.Millisecond)
		return loadErr != nil
	})
	negativeBuilds := builds

	// The failure backoff itself must wake the static view. That rebuild queues
	// the retry; without it a transient failure would leave initials forever.
	time.Sleep(2 * cache.retryAfter)
	waitFor(t, "retry expiry build", func() bool {
		tester.Advance(10 * time.Millisecond)
		return builds > negativeBuilds
	})
	waitFor(t, "retry decode", func() bool { return hasDecoded(cache, rawURL, 16) })
	waitFor(t, "retry success build", func() bool {
		tester.Advance(10 * time.Millisecond)
		return avatarSource != nil
	})
	if got := requests.Load(); got != 2 {
		t.Fatalf("network requests = %d, want initial failure + retry", got)
	}
}
