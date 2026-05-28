# livepaper

**Set per-monitor wallpapers — including live video — from the command line on Windows.**

`livepaper` detects every monitor's real position and resolution, composites your images into a single span wallpaper, and applies it instantly. Drop in a video file instead and it loops as a live wallpaper behind your desktop icons.

---

## Install

```sh
go install github.com/dvgamerr/go-livepaper/cmd/livepaper@latest
```

> **Requirements**
>
> - Windows 10 / 11
> - Go 1.21+
> - [ffmpeg](https://ffmpeg.org/download.html) in `PATH` — only needed for video wallpapers (default renderer)
> - [mpv](https://mpv.io/installation/) in `PATH` — optional alternative video renderer (smoother playback, hardware decode)

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
| Zero config        | No settings file — everything via CLI flags                                          |

---

## Usage

```text
livepaper [--monitor MONITOR] [--clean] [WALLPAPER ...]
```

| Flag           | Short    | Description                                                  |
| -------------- | -------- | ------------------------------------------------------------ |
| `--monitor N`  | `-m N`   | Target monitor by number (1-based). Repeat for each monitor. |
| `--player mpv` | `-p mpv` | Video renderer: `ffmpeg` (default) or `mpv`                  |
| `--clean`      | `-c`     | Delete all temp wallpaper files from `%TEMP%\livepaper`      |
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
4. Saves the canvas as a temporary JPEG in `%TEMP%\livepaper`.
5. Writes `WallpaperStyle=22` (Span) to the registry and calls `SystemParametersInfoW` to apply it.
6. For each video: embeds an ffmpeg-backed GDI window behind the desktop icon layer and loops it at 30 fps.

---

## Build from source

```sh
git clone https://github.com/dvgamerr/go-livepaper.git
cd go-livepaper
mkdir bin
go build -o bin/livepaper.exe ./cmd/livepaper
```

With version embedded:

```sh
mkdir bin
go build -ldflags "-X main.VERSION=$(cat VERSION)" -o bin/livepaper.exe ./cmd/livepaper
```

---

## Video renderers

Two video backends are supported. Choose with `--player`.

|                  | ffmpeg (default)                               | mpv                                    |
| ---------------- | ---------------------------------------------- | -------------------------------------- |
| Availability     | Must be in `PATH`                              | Must be in `PATH`                      |
| Decode           | Software + `hwaccel auto`                      | Hardware (DXVA2/D3D11VA)               |
| Frame delivery   | Raw BGRA pipe → GDI `StretchDIBits`            | Native window embed via `--wid`        |
| CPU usage        | Higher (GDI blit per frame)                    | Lower (GPU compositing)                |
| Playback quality | Good                                           | Better (subtitles, HDR, etc.)          |
| Install          | [ffmpeg.org](https://ffmpeg.org/download.html) | [mpv.io](https://mpv.io/installation/) |

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
