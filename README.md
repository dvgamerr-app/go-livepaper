# livepaper

**Set per-monitor wallpapers — including live video — from the Windows tray app or command line.**

`livepaper` detects every monitor's real position and resolution, composites your images into a single span wallpaper, and applies it instantly. Drop in a video file instead and it loops as a live wallpaper behind your desktop icons.

Live Paper is fully offline: no account, no sign-in, no telemetry, no online gallery, and no
in-app downloads. The app and its installer make no network calls.

---

## Install

Download `livepaper-setup-<version>.exe` from the GitHub Releases page and run it. The installer
copies `livepaper.exe` together with bundled `ffmpeg.exe`, `ffprobe.exe`, and `mpv.exe` into
`%LOCALAPPDATA%\Programs\livepaper\bin`; it does not download anything during installation.

To build the CLI from source instead:

```sh
go install github.com/dvgamerr/go-livepaper/cmd/livepaper@latest
```

> **Requirements**
>
> - Windows 10 / 11
> - Go 1.26.3+ for source builds
> - Bun 1.3.14+ for frontend development
> - `ffmpeg` / `ffprobe` — needed for video wallpapers (default renderer); bundled with the installer
> - `mpv` — optional alternative video renderer (smoother playback, hardware decode); bundled with the installer
>
> At runtime the media tools are found beside `livepaper.exe` or on `PATH`. Source builds can
> bundle them into `bin/` with `scripts/bundle-media-tools.ps1` (see [Installer build](#installer-build)).

---

## Quick start

```sh
# Single monitor — set one image
livepaper "C:\Wallpapers\mountain.jpg"

# Dual monitor — one image per monitor (matched in detection order)
livepaper "C:\Wallpapers\left.jpg" "C:\Wallpapers\right.png"

# Target specific monitors by number
livepaper -m 2 -m 3 "C:\Wallpapers\portrait.jpg" "C:\Wallpapers\stats.png"

# Live video wallpaper on the primary monitor (ffmpeg, default)
livepaper "C:\Wallpapers\rain.mp4"

# Live video wallpaper using mpv instead of ffmpeg
livepaper --player mpv "C:\Wallpapers\rain.mp4"

# Mix: static on monitor 1, video on monitor 2
livepaper -m 1 -m 2 "C:\Wallpapers\left.jpg" "C:\Wallpapers\loop.mp4"

# Clean up temp files
livepaper --clean
```

---

## Features

| Feature            | Details                                                                              |
| ------------------ | ------------------------------------------------------------------------------------ |
| Multi-monitor      | Reads real screen layout from Windows — no manual config                             |
| Per-monitor images | One image per monitor; fill-crop keeps aspect ratio                                  |
| EXIF-aware         | Rotates photos to correct orientation before applying                                |
| Live video         | Loops any video file as a live wallpaper via ffmpeg or mpv                           |
| Mixed mode         | Static images and video on different monitors simultaneously                         |
| Formats            | Images: `jpg` `jpeg` `png` · Video: `mp4` `mkv` `avi` `mov` `webm` `m4v` `flv` `gif` |
| Tray workspace     | Assign, preview, pause, and restore wallpapers from the Wails desktop UI             |
| Library            | Recently applied wallpapers, kept locally on this machine                            |
| Global hotkeys     | Next, previous, play/pause, and open the workspace from anywhere                     |
| Power aware        | Optionally pause live wallpapers on battery saver or behind a fullscreen app         |
| CLI automation     | Apply wallpapers directly from scripts without opening the tray workspace            |
| Offline            | No account, telemetry, or network access; media tools ship with the installer        |

---

## Usage

```text
livepaper [--monitor MONITOR] [--clean] [WALLPAPER ...]
```

| Flag           | Short    | Description                                                  |
| -------------- | -------- | ------------------------------------------------------------ |
| `--monitor N`  | `-m N`   | Target monitor by number (1-based). Repeat for each monitor. |
| `--player mpv` | `-p mpv` | Video renderer: `ffmpeg` (default) or `mpv`                  |
| `--clean`      | `-c`     | Delete all temp wallpaper files from `<install dir>\data`      |
| `--version`    |          | Print version                                                |
| `--help`       | `-h`     | Print help                                                   |

**Monitor matching rules**

- Omit `-m` → images are assigned to monitors in the order Windows enumerates them (primary monitor is usually monitor 1).
- Use `-m` → the count of `-m` flags must equal the count of wallpaper paths. Each path maps to its corresponding `-m` value.
- Monitor numbers start at `1`. Run `livepaper --help` to see how many monitors are detected.

---

## How it works

1. Queries `EnumDisplayMonitors` to get every monitor's position and resolution.
2. Creates a black canvas sized to the full virtual desktop.
3. For each image: loads it, applies EXIF rotation, fill-crops it to the monitor size, and draws it onto the canvas at the correct position.
4. Saves the canvas as a temporary JPEG in `<install dir>\data`.
5. Writes `WallpaperStyle=22` (Span) to the registry and calls `SystemParametersInfoW` to apply it.
6. For each video: embeds an ffmpeg-backed GDI window behind the desktop icon layer and loops it at 30 fps.

---

## Build from source

```sh
git clone https://github.com/dvgamerr/go-livepaper.git
cd go-livepaper
bun install
bun run build
mkdir bin
go build -tags production -o bin/livepaper.exe ./cmd/livepaper
```

The production Go binary embeds `cmd/livepaper/dist`, so rebuild the Astro frontend before a
production binary whenever frontend assets change. `-tags production` compiles out Wails'
development-server probe, so the binary makes no network calls.

With version embedded:

```sh
mkdir bin
go build -tags production -ldflags "-X main.VERSION=$(cat VERSION)" -o bin/livepaper.exe ./cmd/livepaper
```

### Development and validation

Development uses two terminals. Do not run the production frontend build before `wails3 dev`;
the development build proxies Astro directly.

```sh
# Terminal 1
bun run dev

# Terminal 2
wails3 dev
```

Run the automated checks before shipping:

```sh
bun run check
bun run format
bun run build
go test ./...
go vet ./...
go build -o "$TEMP/livepaper-check.exe" ./cmd/livepaper
git diff --check
```

See [`AGENTS.md`](AGENTS.md) for build constraints and
[`docs/project-structure.md`](docs/project-structure.md) for the frontend/backend runtime flow.

### Installer build

`scripts\installer.bat` builds the full offline installer: icons, the Astro frontend, the Windows
resource, `bin\livepaper.exe`, the bundled media tools, and finally `installer\livepaper.nsi`.

The media tools are fetched once, at build time, by `scripts/bundle-media-tools.ps1`:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\bundle-media-tools.ps1 -OutputDir bin
```

It downloads `ffmpeg.exe` + `ffprobe.exe` (BtbN/FFmpeg-Builds, win64 GPL) and `mpv.exe`
(shinchiro/mpv-winbuild-cmake, x86_64) into `bin\`, copies upstream license files and a source
note into `bin\licenses`, and exits non-zero if anything is missing. It only checks `bin\`, never
the build machine's `PATH`; pass `-Force` to re-download. The release workflow runs the same
script before `makensis`, and the NSIS script packages exactly `livepaper.exe`, the three tools,
and `licenses\`.

---

## Video renderers

Two video backends are supported. Choose with `--player`.

|                  | ffmpeg (default)                             | mpv                                          |
| ---------------- | -------------------------------------------- | -------------------------------------------- |
| Availability     | Bundled beside `livepaper.exe`, or on `PATH` | Bundled beside `livepaper.exe`, or on `PATH` |
| Decode           | Software + `hwaccel auto`                    | Hardware (DXVA2/D3D11VA)                     |
| Frame delivery   | Raw BGRA pipe → GDI `StretchDIBits`          | Native window embed via `--wid`              |
| CPU usage        | Higher (GDI blit per frame)                  | Lower (GPU compositing)                      |
| Playback quality | Good                                         | Better (subtitles, HDR, etc.)                |
| Bundled build    | BtbN/FFmpeg-Builds (win64 GPL)               | shinchiro/mpv-winbuild-cmake (x86_64)        |

**When to use mpv** — prefer `--player mpv` when you have a high-resolution or high-framerate video, or when ffmpeg causes visible CPU load. mpv renders directly into the desktop shell layer using its `--wid` embedding flag; no frame piping is needed.

```sh
# Verify mpv is available
mpv --version

# Use mpv for a 4K wallpaper loop
livepaper --player mpv "C:\Wallpapers\4k-loop.mp4"
```

---

## Limitations

- Windows only — uses `user32.dll`, `gdi32.dll`, and the Windows registry directly.
- The final wallpaper is always a single composited JPEG (Span mode). Windows per-monitor wallpaper APIs are not used.
- Monitors without an assigned wallpaper appear black.
- Video live wallpaper currently targets one monitor per video instance; the video loops indefinitely until the process is killed.
- JPEG output has minor quality loss compared to a PNG source (`quality=90`).

---

## License

MIT

### Third-party software

The installer redistributes unmodified third-party binaries: `ffmpeg.exe` and `ffprobe.exe` from
[FFmpeg](https://ffmpeg.org/) and `mpv.exe` from [mpv](https://mpv.io/). The bundled builds are
GPL-licensed and are not covered by the MIT license above. Their upstream license files and
source notes are installed under `bin\licenses`.
