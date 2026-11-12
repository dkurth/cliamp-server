package ignore

import (
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

func TestRel(t *testing.T) {
	if r, ok := Rel("/m/lib", "/m/lib/A/b.mp3"); !ok || r != "A/b.mp3" {
		t.Errorf("got %q %v", r, ok)
	}
	if _, ok := Rel("/m/lib", "/m/ads/x.mp3"); ok {
		t.Error("outside root should be false")
	}
}
