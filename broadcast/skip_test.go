package broadcast

import "testing"

func TestSkipCurrentIf(t *testing.T) {
	h := NewHub("radio", nil, 64, 0)
	h.currentTrack.Store(TrackInfo{Path: "/lib/a.mp3"})

	h.SkipCurrentIf(func(string) bool { return false })
	if h.skipRequested("/lib/a.mp3") {
		t.Fatal("skipped although the check said no")
	}

	h.SkipCurrentIf(func(p string) bool { return p == "/lib/a.mp3" })
	if !h.skipRequested("/lib/a.mp3") {
		t.Fatal("expected the playing track to be skipped")
	}
	if h.skipRequested("/lib/a.mp3") {
		t.Fatal("one request should skip once")
	}

	// A request that arrives as the track ends must not skip the next one.
	h.SkipCurrentIf(func(string) bool { return true })
	if h.skipRequested("/lib/b.mp3") {
		t.Fatal("a stale request skipped the next track")
	}
	if h.skipRequested("/lib/a.mp3") {
		t.Fatal("the stale request should have been dropped")
	}
}
