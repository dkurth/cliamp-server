package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cliamp-server/broadcast"
	"cliamp-server/ignore"
)

// Status handles GET /{station}/status — JSON per-station status.
type Status struct {
	Hub         *broadcast.Hub
	StartTime   time.Time
	Password    string
	TrackCount  int
	StationName string
	LibraryRoot string // used to report track paths relative to the library
}

type recentTrackResponse struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Path     string `json:"path,omitempty"` // relative to the library root
	PlayedAt string `json:"played_at"`
}

type statusResponse struct {
	Station         string                `json:"station"`
	Listeners       int                   `json:"listeners"`
	ListenerDetails []listenerDetail      `json:"listener_details"`
	CurrentTrack    currentTrack          `json:"current_track"`
	RecentTracks    []recentTrackResponse `json:"recent_tracks"`
	Uptime          string                `json:"uptime"`
	UptimeSeconds   int64                 `json:"uptime_seconds"`
	PlaylistLength  int                   `json:"playlist_length"`
}

type currentTrack struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Path   string `json:"path,omitempty"` // relative to the library root
}

type listenerDetail struct {
	IP              string  `json:"ip"`
	Country         string  `json:"country,omitempty"`
	CountryCode     string  `json:"country_code,omitempty"`
	City            string  `json:"city,omitempty"`
	Latitude        float64 `json:"latitude,omitempty"`
	Longitude       float64 `json:"longitude,omitempty"`
	ConnectedAt     string  `json:"connected_at"`
	DurationSeconds int64   `json:"duration_seconds"`
}

func (s *Status) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Password != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.Password {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	track := s.Hub.CurrentTrack()
	uptime := time.Since(s.StartTime)
	now := time.Now()

	resp := statusResponse{
		Station:         s.StationName,
		Listeners:       s.Hub.ListenerCount(),
		ListenerDetails: buildListenerDetails(s.Hub.Listeners(), now),
		CurrentTrack: currentTrack{
			Title:  track.Title,
			Artist: track.Artist,
			Album:  track.Album,
			Path:   relPath(s.LibraryRoot, track.Path),
		},
		RecentTracks:   buildRecentTracks(s.Hub.RecentTracks(), s.LibraryRoot),
		Uptime:         formatDuration(uptime),
		UptimeSeconds:  int64(uptime.Seconds()),
		PlaylistLength: s.TrackCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// StationInfo holds the data needed for global status aggregation.
type StationInfo struct {
	Hub         *broadcast.Hub
	Name        string
	TrackCount  int
	LibraryRoot string
}

// GlobalStatus handles GET /status — aggregated status for all stations.
type GlobalStatus struct {
	Stations  map[string]*StationInfo
	StartTime time.Time
	Password  string
}

type globalStatusResponse struct {
	Stations       map[string]stationStatus `json:"stations"`
	TotalListeners int                      `json:"total_listeners"`
	Uptime         string                   `json:"uptime"`
	UptimeSeconds  int64                    `json:"uptime_seconds"`
}

type stationStatus struct {
	Name            string                `json:"name"`
	Listeners       int                   `json:"listeners"`
	ListenerDetails []listenerDetail      `json:"listener_details"`
	CurrentTrack    currentTrack          `json:"current_track"`
	RecentTracks    []recentTrackResponse `json:"recent_tracks"`
	PlaylistLength  int                   `json:"playlist_length"`
}

func (g *GlobalStatus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.Password != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != g.Password {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	uptime := time.Since(g.StartTime)
	now := time.Now()
	stations := make(map[string]stationStatus, len(g.Stations))
	totalListeners := 0

	for id, info := range g.Stations {
		track := info.Hub.CurrentTrack()
		listeners := info.Hub.ListenerCount()
		totalListeners += listeners

		stations[id] = stationStatus{
			Name:            info.Name,
			Listeners:       listeners,
			ListenerDetails: buildListenerDetails(info.Hub.Listeners(), now),
			CurrentTrack: currentTrack{
				Title:  track.Title,
				Artist: track.Artist,
				Album:  track.Album,
				Path:   relPath(info.LibraryRoot, track.Path),
			},
			RecentTracks:   buildRecentTracks(info.Hub.RecentTracks(), info.LibraryRoot),
			PlaylistLength: info.TrackCount,
		}
	}

	resp := globalStatusResponse{
		Stations:       stations,
		TotalListeners: totalListeners,
		Uptime:         formatDuration(uptime),
		UptimeSeconds:  int64(uptime.Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// relPath reports p relative to the library root, or "" if it isn't inside
// it (intro and ad files) or no root is known.
func relPath(root, p string) string {
	rel, ok := ignore.Rel(root, p)
	if !ok {
		return ""
	}
	return rel
}

func buildRecentTracks(tracks []broadcast.RecentTrack, root string) []recentTrackResponse {
	out := make([]recentTrackResponse, len(tracks))
	for i, t := range tracks {
		out[i] = recentTrackResponse{
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Path:     relPath(root, t.Path),
			PlayedAt: t.PlayedAt.UTC().Format(time.RFC3339),
		}
	}
	return out
}

func buildListenerDetails(snaps []broadcast.ListenerSnapshot, now time.Time) []listenerDetail {
	details := make([]listenerDetail, len(snaps))
	for i, snap := range snaps {
		details[i] = listenerDetail{
			IP:              snap.Info.IP,
			Country:         snap.Info.Country,
			CountryCode:     snap.Info.CountryCode,
			City:            snap.Info.City,
			Latitude:        snap.Info.Latitude,
			Longitude:       snap.Info.Longitude,
			ConnectedAt:     snap.ConnectedAt.UTC().Format(time.RFC3339),
			DurationSeconds: int64(now.Sub(snap.ConnectedAt).Seconds()),
		}
	}
	return details
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}
