# Configuration

There are two ways to configure cliamp-server: CLI flags for a quick single station, or a TOML config file for multi-station setups.

## CLI Flags

```
./cliamp-server --music /path/to/mp3s --shuffle --port 8000
```

| Flag | Description | Default |
|------|-------------|---------|
| `--music <path>` | Path to audio directory (creates station "radio") | |
| `--port <port>` | Listen port | 8000 |
| `--shuffle` | Enable shuffle mode | on |
| `--no-shuffle` | Disable shuffle mode | |
| `--name <name>` | Station name | "cliamp radio" |
| `--intro <path>` | Intro MP3 played once when a listener connects | |
| `--ads <path>` | Ads directory or single MP3 file | |
| `--ignore-file <path>` | File listing tracks never to play (see below) | |
| `--ad-every-songs <n>` | Insert an ad after every N songs | 0 (off) |
| `--ad-every-minutes <n>` | Insert an ad after every N minutes | 0 (off) |
| `--password <token>` | Admin password for /status endpoints | |
| `--geo-db <path>` | Path to MaxMind GeoLite2-City.mmdb file | |
| `--stats-db <path>` | Path to SQLite database for listener statistics | |
| `--log-level <level>` | Log level: debug, info, warn, error | info |

## Config File

The server reads `~/.config/cliamp-server/config.toml` automatically. CLI flags override config file values when both are present.

```toml
[server]
host = "0.0.0.0"
port = 8000

[stream]
metaint = 8192       # ICY metadata interval in bytes
buffer_size = 512    # Ring buffer size in KB (512 ~ 32s of 128kbps audio)

[admin]
password = ""        # Bearer token for /status endpoints (empty = open)

[log]
level = "info"

[geo]
db_path = ""         # Path to MaxMind GeoLite2-City.mmdb (optional)

[stats]
db_path = ""         # Path to SQLite database for listener statistics (optional)

[stations.pop]
name = "Pop Station"
description = "Pop Music 24/7"
genre = "Pop"
url = ""
path = "/music/pop"
shuffle = true
recursive = true

[stations.jazz]
name = "Jazz Station"
description = "Smooth Jazz"
genre = "Jazz"
url = ""
path = "/music/jazz"
shuffle = true
recursive = true
```

Each `[stations.<id>]` block creates a station. The `<id>` determines the URL prefix: `[stations.pop]` becomes `/pop/stream`.

## Station Options

Every station supports these fields:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Display name shown in ICY metadata |
| `description` | string | Station description |
| `genre` | string | Genre tag sent in ICY headers |
| `url` | string | Station URL sent in ICY headers |
| `path` | string | **Required.** Directory containing audio files |
| `shuffle` | bool | Randomize playback order |
| `recursive` | bool | Scan subdirectories for audio files |
| `intro_file` | string | MP3 played once per listener before joining the live broadcast |
| `ads_path` | string | Directory of ad MP3 files (or a single file) |
| `ad_every_n_songs` | int | Play an ad after every N music tracks |
| `ad_every_n_minutes` | int | Play an ad after every N minutes |
| `ad_shuffle` | bool | Randomize ad selection order |
| `ignore_file` | string | File listing tracks never to play (see below) |

## Ignore File

`ignore_file` (or `--ignore-file`) points at a plain-text file of tracks to skip.
Rules are matched against each track's path relative to the station's `path`,
with forward slashes. The file is re-read whenever it changes, so no restart is
needed. A missing file ignores nothing.

```
# comments and blank lines are skipped
Artist/Album/03 Song.mp3     exact path
Some Artist/                 everything under a folder (trailing slash)
*Live*                       glob on the file name (no slash in the rule)
Other/*.wma                  glob on the full relative path
```

The track after the one currently being prepared is the first that can be
affected by an edit, so a change takes effect within a track or two.
