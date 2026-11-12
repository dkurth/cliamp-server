// Package ignore skips tracks listed in an ignore file.
//
// The file is plain text, one rule per line, matched against each track's path
// relative to the station's library root (always with forward slashes):
//
//	# comment
//	Artist/Album/03 Song.mp3     exact path
//	Some Artist/                 everything under a folder (trailing slash)
//	*Live*                       glob (path.Match) against the full relative path
//	                             or, if the rule has no slash, the file name
//	Artist/\[Live\] Song.mp3     backslash escapes * ? [ \ (and a leading #)
//
// The file is re-read whenever its modification time or size changes, so edits
// take effect without restarting the server, and a track that becomes ignored
// while it's playing is cut short (see Filter.Ignored and Hub.SetSkipCheck). A missing file means "ignore
// nothing".
package ignore

import (
	"bufio"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cliamp-server/library"
	"cliamp-server/playlist"
)

// Rules is a parsed ignore file.
type Rules struct {
	exact    map[string]bool
	prefixes []string
	globs    []string
}

// Parse reads rules from the file contents.
func Parse(data string) *Rules {
	r := &Rules{exact: map[string]bool{}}
	sc := bufio.NewScanner(strings.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(filepath.ToSlash(line), "/")
		switch {
		case strings.HasSuffix(line, "/"):
			r.prefixes = append(r.prefixes, line)
		case strings.ContainsAny(line, `*?[\`):
			r.globs = append(r.globs, line)
		default:
			r.exact[line] = true
		}
	}
	return r
}

// Len is the number of rules.
func (r *Rules) Len() int {
	if r == nil {
		return 0
	}
	return len(r.exact) + len(r.prefixes) + len(r.globs)
}

// Match reports whether the relative path rel is ignored.
func (r *Rules) Match(rel string) bool {
	if r == nil {
		return false
	}
	if r.exact[rel] {
		return true
	}
	for _, p := range r.prefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	base := path.Base(rel)
	for _, g := range r.globs {
		target := rel
		if !strings.Contains(g, "/") {
			target = base
		}
		if ok, _ := path.Match(g, target); ok {
			return true
		}
	}
	return false
}

// Filter wraps a TrackSource and skips tracks matched by the ignore file.
type Filter struct {
	src      playlist.TrackSource
	root     string
	file     string
	maxSkips int

	mu      sync.Mutex
	rules   *Rules
	modTime time.Time
	size    int64
	loaded  bool
}

// NewFilter returns a TrackSource that draws from src and skips ignored
// tracks. root is the library root used to make paths relative; file is the
// ignore file. maxSkips bounds how many tracks are skipped in a row (use the
// library size) so an ignore-everything file can't spin forever.
func NewFilter(src playlist.TrackSource, root, file string, maxSkips int) *Filter {
	if maxSkips < 1 {
		maxSkips = 1
	}
	f := &Filter{src: src, root: root, file: file, maxSkips: maxSkips}
	f.reload()
	return f
}

// Next returns the next track that isn't ignored. If every track tried is
// ignored, it gives up and returns the last one rather than stalling the
// stream.
func (f *Filter) Next() library.Track {
	rules := f.reload()
	var t library.Track
	for i := 0; i < f.maxSkips; i++ {
		t = f.src.Next()
		rel, ok := Rel(f.root, t.Path)
		if !ok || !rules.Match(rel) {
			return t
		}
		slog.Info("skipping ignored track", "path", rel)
	}
	slog.Warn("every track tried is ignored; playing one anyway", "path", t.Path)
	return t
}

// Ignored reports whether the track at absolute path p is ignored right now
// (re-reading the file first if it changed). The hub uses it to cut short a
// track that was ignored while it was playing.
func (f *Filter) Ignored(p string) bool {
	rel, ok := Rel(f.root, p)
	return ok && f.reload().Match(rel)
}

// reload re-reads the ignore file if it changed and returns the current rules.
func (f *Filter) reload() *Rules {
	f.mu.Lock()
	defer f.mu.Unlock()

	info, err := os.Stat(f.file)
	if err != nil {
		if f.rules.Len() > 0 || !f.loaded {
			if !os.IsNotExist(err) {
				slog.Warn("cannot read ignore file", "path", f.file, "error", err)
			}
			f.rules = nil
			f.modTime, f.size = time.Time{}, 0
		}
		f.loaded = true
		return f.rules
	}
	if f.loaded && info.ModTime().Equal(f.modTime) && info.Size() == f.size {
		return f.rules
	}
	data, err := os.ReadFile(f.file)
	if err != nil {
		slog.Warn("cannot read ignore file", "path", f.file, "error", err)
		return f.rules
	}
	f.rules = Parse(string(data))
	f.modTime, f.size, f.loaded = info.ModTime(), info.Size(), true
	slog.Info("loaded ignore file", "path", f.file, "rules", f.rules.Len())
	return f.rules
}

// Rel returns p relative to root with forward slashes, and false if p is not
// inside root.
func Rel(root, p string) (string, bool) {
	if root == "" || p == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
