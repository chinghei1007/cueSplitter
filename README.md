# Cue Splitter

Cue Splitter is a local, single-port web application. A Go server handles album jobs and serves an embedded Vite/React production build from the same executable.

## What it accepts

Drop a folder or select individual files containing:

- exactly one `.cue` file;
- exactly one supported audio file (`aac`, `aiff`, `ape`, `flac`, `m4a`, `mp3`, `ogg`, `opus`, `wav`, or `wma`);
- an optional cover (`jpg`, `jpeg`, `png`, or `webp`); and
- an optional `.txt` file with one numbered track name per non-empty line, such as `01. Song title`.

The name source can be switched between the TXT song list, CUE `TITLE` fields, and generated `track01`, `track02`, and so on. TXT and CUE-title decoding supports Auto/UTF-8, Big5, GB18030, and Shift_JIS. Auto accepts a UTF-8 BOM or valid UTF-8 and asks for a manual legacy encoding when necessary.

Only the selected naming source is decoded and validated. An optional TXT cannot block a CUE-based job, but selecting TXT immediately applies its decoding, numbered-line, sequence, and track-count checks in both the browser and Go server.

TXT lines must be sequential and match the CUE track count. Accepted separators are `.`, `)`, `:`, `-`, `–`, `—`, and `、`, followed by at least one space. The number and separator are removed from editable titles because the track rows already show numbering. The Go server repeats the validation so a direct API request cannot bypass it.

If no song list exists, Cue Splitter uses complete CUE `TITLE` fields or generated `trackNN` names. Every successful export includes a normalized UTF-8-with-BOM `tracklist.txt` in `01. Title` form. The supplied image is normalized once to `cover.jpg`; without one, the output uses a blank white JPEG.

CUE album-level `TITLE` and `PERFORMER` values prefill editable **Album** and **Artist** fields. Exported audio receives title, track number, album, artist, and album-artist tags.

The same `cover.jpg` remains beside the tracks and is embedded as the front-cover attached picture in every FLAC, MP3, and M4A track. WAV, AIFF, raw AAC, OGG, Opus, and WMA keep the compatibility sidecar because those containers do not reliably expose FFmpeg attached pictures to players.

## Development setup

The tools are separated in the familiar frontend/backend style:

- `frontend/` contains Vite, React, TypeScript, and UI components;
- `cmd/` and `internal/` contain the Go server, job manager, CUE parser, and FFmpeg runner; and
- `web/dist/` is the frontend production build embedded by Go.

### One-command setup

On macOS or Linux:

```sh
./scripts/setup.sh
```

On Windows PowerShell:

```powershell
.\scripts\setup.ps1
```

The setup script checks for Go, Node.js/npm, FFmpeg, and FFprobe. It installs only missing tools using Homebrew on macOS, a supported system package manager on Linux (`apt`, `dnf`, `yum`, `pacman`, `apk`, or `zypper`), or `winget` on Windows. It does not download or execute an unknown package-manager installer; if Homebrew or `winget` itself is missing, it explains what must be installed first.

Use check-only mode when you do not want package changes:

```sh
./scripts/setup.sh --check
```

For an already compiled release, only FFmpeg/FFprobe are runtime dependencies:

```sh
./scripts/setup.sh --runtime-only
```

Build and run:

```sh
./scripts/build.sh
./bin/cue-splitter
```

`build.sh` runs the dependency check, installs missing tools through the setup script when necessary, builds React, and writes the single application executable to `bin/`. npm is a build dependency only; the built Go executable embeds the frontend and does not run npm.

Then open <http://127.0.0.1:8080>.

During frontend development, run the Go server in one terminal and Vite in another:

```sh
go run ./cmd/cuesplitter
npm --prefix frontend run dev
```

Vite proxies `/api` to Go. Production uses only the Go port.

## Output destinations

- **Browser Downloads · ZIP** streams a ZIP to the browser's configured download folder.
- **Project root `/output`** writes to `output/<album-name>` under the directory where the server was started.
- **Other server path** accepts an absolute path or a path relative to the server working directory.

Existing album directories are not overwritten; Cue Splitter adds a short job suffix.

Both Format and Bit rate include **Original**. Original format preserves the source container/format while FFmpeg re-encodes when needed for precise CUE boundaries. Original bitrate uses the detected source bitrate for lossy outputs; FLAC, WAV, and AIFF remain lossless. APE can be read as input, but its Original export option is unavailable because FFmpeg does not provide a compatible APE encoder; select FLAC or WAV instead.

## Cross-platform releases

End users do not run `go install`. They receive one compiled executable plus the FFmpeg runtime dependency. The executable first looks for `ffmpeg` and `ffprobe` beside itself, then falls back to `PATH`, so a release can be a self-contained folder containing three platform-matched binaries.

Build platform-specific binaries with Go environment variables after the frontend has been built:

```sh
GOOS=windows GOARCH=amd64 go build -o bin/cue-splitter.exe ./cmd/cuesplitter
GOOS=linux GOARCH=amd64 go build -o bin/cue-splitter-linux-amd64 ./cmd/cuesplitter
GOOS=darwin GOARCH=arm64 go build -o bin/cue-splitter-darwin-arm64 ./cmd/cuesplitter
```

The React frontend is already embedded in each binary.

## Checks

```sh
npm --prefix frontend run typecheck
go test ./...
go vet ./...
make e2e
```

The direct E2E script also uses `curl`, `jq`, and `iconv`. It starts an isolated server, checks selected and unselected source behavior, runs UTF-8/GB18030/Big5/Shift_JIS/generated-name exports, inspects normalized TXT and audio metadata, and removes its temporary files afterward.
