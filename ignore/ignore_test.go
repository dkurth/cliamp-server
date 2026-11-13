package ignore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cliamp-server/library"
)

func TestMatch(t *testing.T) {
	r := Parse(`
# comment
Artist/Album/03 Song.mp3
Bad Band/
*Live*
Other/*.wma
X/\[Live\] One.mp3
\#1 Hit.mp3
Y/\[Demo] \*.mp3
`)
	cases := map[string]bool{
		"Artist/Album/03 Song.mp3": true,
		"Artist/Album/04 Song.mp3": false,
		"Bad Band/Album/x.mp3":     true,
		"Bad Bandit/x.mp3":         false,
		"A/B/Song (Live).mp3":      true,
		"Other/x.wma":              true,
		"Other/sub/x.wma":          false,
		"Somewhere/Other/x.wma":    false,
		"# comment":                false,
		"X/[Live] One.mp3":         true,
		"X/L One.mp3":              false,
		"#1 Hit.mp3":               true,
		"Y/[Demo] *.mp3":           true,
		"Y/[Demo] x.mp3":           false,
	}
	for p, want := range cases {
		if got := r.Match(p); got != want {
			t.Errorf("Match(%q) = %v, want %v", p, got, want)
		}
	}
	if r.Len() != 7 {
		t.Errorf("Len = %d, want 7", r.Len())
	}
}

type seqSource struct {
	tracks []library.Track
	i      int
}

func (s *seqSource) Next() library.Track {
	t := s.tracks[s.i%len(s.tracks)]
	s.i++
	return t
}

func TestFilterSkipsAndReloads(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "lib")
	file := filepath.Join(dir, "radio_ignore")
	src := &seqSource{tracks: []library.Track{
		{Path: filepath.Join(root, "a.mp3")},
		{Path: filepath.Join(root, "b.mp3")},
		{Path: filepath.Join(root, "c.mp3")},
	}}

	// No file: nothing ignored.
	f := NewFilter(src, root, file, 3)
	if got := f.Next().Path; got != filepath.Join(root, "a.mp3") {
		t.Fatalf("got %s", got)
	}

	// Ignore b: the next call skips it.
	if err := os.WriteFile(file, []byte("b.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := f.Next().Path; got != filepath.Join(root, "c.mp3") {
		t.Fatalf("expected c, got %s", got)
	}

	// Ignore everything: gives up after maxSkips and still returns a track.
	if err := os.WriteFile(file, []byte("*.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	os.Chtimes(file, future, future)
	if got := f.Next().Path; got == "" {
		t.Fatal("expected a track")
	}

	// File removed: nothing ignored again.
	os.Remove(file)
	if got := f.Next(); got.Path == "" {
		t.Fatal("expected a track")
	}
}

func TestIgnoredPicksUpChanges(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "lib")
	file := filepath.Join(dir, "radio_ignore")
	f := NewFilter(&seqSource{tracks: []library.Track{{Path: filepath.Join(root, "a.mp3")}}}, root, file, 1)
	playing := filepath.Join(root, "Artist", "a.mp3")

	if f.Ignored(playing) {
		t.Fatal("no file: nothing should be ignored")
	}
	if err := os.WriteFile(file, []byte("Artist/a.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !f.Ignored(playing) {
		t.Fatal("expected the playing track to be ignored once it's in the file")
	}
	if f.Ignored(filepath.Join(root, "Artist", "b.mp3")) {
		t.Fatal("other tracks should still play")
	}
	if f.Ignored(filepath.Join(dir, "ads", "ad.mp3")) {
		t.Fatal("tracks outside the library (ads) are never ignored")
	}
}

func TestWatchReportsChanges(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "lib")
	file := filepath.Join(dir, "radio_ignore")
	f := NewFilter(&seqSource{tracks: []library.Track{{Path: filepath.Join(root, "a.mp3")}}}, root, file, 1)
	playing := filepath.Join(root, "a.mp3")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := make(chan bool, 10)
	go f.Watch(ctx, func() { changed <- f.Ignored(playing) })
	time.Sleep(100 * time.Millisecond) // let the watch start

	wait := func(what string) bool {
		select {
		case v := <-changed:
			return v
		case <-time.After(3 * time.Second):
			t.Fatalf("no change reported after %s", what)
			return false
		}
	}

	// Replaced with a rename, the way radio-app saves it.
	tmp := filepath.Join(dir, ".radio_ignore-tmp")
	if err := os.WriteFile(tmp, []byte("a.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, file); err != nil {
		t.Fatal(err)
	}
	if !wait("rename") {
		t.Fatal("expected a.mp3 to be ignored after the rename")
	}

	// Edited in place.
	if err := os.WriteFile(file, []byte("# nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if wait("write") {
		t.Fatal("expected a.mp3 to play again after the edit")
	}

	// Other files in the folder don't count.
	if err := os.WriteFile(filepath.Join(dir, "other"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changed:
		t.Fatal("a change to another file was reported")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRel(t *testing.T) {
	if r, ok := Rel("/m/lib", "/m/lib/A/b.mp3"); !ok || r != "A/b.mp3" {
		t.Errorf("got %q %v", r, ok)
	}
	if _, ok := Rel("/m/lib", "/m/ads/x.mp3"); ok {
		t.Error("outside root should be false")
	}
}
