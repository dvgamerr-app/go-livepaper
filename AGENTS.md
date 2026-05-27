## Agent instructions

**Every time you try something and it is wrong, record it immediately in `docs/wallpaper-internals.md` under "Known Mistakes".** Do not attempt the same approach twice. Read that section at the start of any session involving `desktop.go` or `video.go` before writing any code.

Deep references:

- `docs/project-structure.md` for folder structure, frameworks, and runtime flow
- `docs/wallpaper-internals.md` for Win32/wallpaper/video internals

## Build

```nu
let version = (open VERSION | str trim)
^go build -ldflags $"-X main.VERSION=($version)" -o livepaper.exe ./cmd/livepaper/
```

## Run / Test

There is no test suite. Manual testing requires a Windows machine with multiple monitors. Verify by running the built binary directly:

```nu
.\livepaper.exe 'C:\Wallpapers\test.jpg'
```

## Frameworks And Runtime

- Go `1.26.3`
- Wails `v3.0.0-alpha.96`
- Astro `6.3.8`
- Win32 APIs via `golang.org/x/sys` and `syscall`
- `ffmpeg` / `ffprobe` / `mpv` for video processing and playback
- Bun as the default JavaScript runtime for repo scripts

## Project Structure

- `cmd/livepaper/main.go` — CLI entrypoint; no args = tray mode, args = wallpaper apply mode
- `cmd/livepaper/tray.go` — Wails app bootstrap, tray, hidden window lifecycle
- `cmd/livepaper/service.go` — bridge ระหว่าง frontend กับ `internal/wallpaper`
- `internal/wallpaper/*.go` — monitor detection, image compose, video pipeline, Win32 desktop embedding
- `src/pages/index.astro` — tray UI หลักและ browser-side flow
- `src/components/*` / `src/styles/app.css` — UI pieces และ styling
- `public/wails/runtime.js` — runtime bridge ที่ frontend ใช้เรียก Go service
- `cmd/livepaper/assets_dev.go` / `assets_prod.go` — dev proxy กับ prod embedded assets
- `scripts/generate-icons.js` / `install-deps.ps1` — icon generation และ dependency setup
- `docs/*.md` — เอกสารอ้างอิงเชิงลึกที่ไม่ควรใส่ซ้ำใน prompt หลัก

## System Flow

### Startup

1. `main.go` parse args และเลือก mode
2. ถ้า `--clean` ให้ลบ `%TEMP%\livepaper`
3. ถ้าไม่มี wallpaper args ให้เข้า tray mode
4. ถ้ามี args ให้เข้า CLI apply flow

### CLI apply flow

1. `main.go` เรียก `GetMonitors()` และเลือก monitor เป้าหมาย
2. image จะถูก `LoadAndResizeImage()` แล้ว draw ลง canvas เดียว
3. video จะถูก `PreprocessVideo()` แล้วเก็บเป็น `VideoTarget`
4. ถ้ามี image หรือ video ให้ save composite JPEG และ `SetWallpaper()`
5. ถ้ามี video ให้ `RunVideoWallpapers()` เพื่อ embed playback บน desktop

### Tray apply flow

1. `tray.go` สร้าง Wails app, tray, และ hidden frameless window
2. `index.astro` โหลด monitors, version, dependency status, และ restore state เก่า
3. frontend เรียก `AppService` ผ่าน `runtime.js` เพื่อ browse file, generate thumbnail, และ preprocess video
4. เมื่อกด Apply, frontend ส่ง assignments ไป `ApplyWallpapers()`
5. `service.go` compose canvas, set static wallpaper, แล้ว start video wallpapers แบบ background goroutine

### Asset delivery

- dev: `assets_dev.go` proxy ไป Astro dev server `http://localhost:4321`
- prod: `assets_prod.go` serve embedded `dist/`

## Key Constraints

- Windows-only
- final wallpaper is always one composite JPEG in `Span` mode
- monitor slots without assigned media render black
- manual testing is required; there is no automated test suite
- live wallpaper internals are sensitive on Windows 11 24H2; read `docs/wallpaper-internals.md#known-mistakes` before touching `desktop.go` or `video.go`
