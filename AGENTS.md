## Agent instructions

**Every time you try something and it is wrong, record it immediately in `docs/wallpaper-internals.md` under "Known Mistakes".** Do not attempt the same approach twice. Read that section at the start of any session involving `desktop.go` or `video.go` before writing any code.

Deep references:

- `docs/project-structure.md` for folder structure, frameworks, and runtime flow
- `docs/wallpaper-internals.md` for Win32/wallpaper/video internals

## Build

```nu
let version = (try { ^git describe --tags --abbrev=0 | str trim | str replace --regex '^v' '' } catch { "dev" })
mkdir bin
^go build -ldflags $"-X main.VERSION=($version)" -o bin/livepaper.exe ./cmd/livepaper/
```

Frontend (production only):

```sh
bun run build
```

Installer payload (build machine only; the app and installer stay offline):

```sh
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/bundle-media-tools.ps1 -OutputDir bin
```

- `scripts/bundle-media-tools.ps1` downloads `ffmpeg.exe`, `ffprobe.exe`, and `mpv.exe` into `bin/` plus license files/source notes into `bin/licenses`; it checks only the output directory, never the build machine's `PATH`, and exits non-zero if anything is missing
- `scripts/installer.bat` and `.github/workflows/release.yml` run the bundler before `makensis`; `installer/livepaper.nsi` packages exactly `livepaper.exe`, the three tools, and `licenses/`, and must never run scripts or download at install time
- Shipped binaries build with `-tags production`, which removes Wails' `FRONTEND_DEVSERVER_URL` HTTP probe and dev asset proxy from the exe; `wails3 dev` keeps using only `-tags livepaper_dev`

## Dev Mode (`wails3 dev`)

**Do not run `bun run build` before `wails3 dev`.** Dev mode proxies the frontend from the Astro dev server — it does NOT embed or build the frontend.

Workflow (run in two separate terminals):

```sh
# Terminal 1 — Astro dev server (port 9245)
bun run dev

# Terminal 2 — Wails dev (builds Go with livepaper_dev tag, proxies to :9245)
wails3 dev
```

- `assets_dev.go` (build tag `livepaper_dev`) proxies all asset requests to `http://localhost:9245`
- `build/config.yml` excludes `bun run build` and passes `-tags livepaper_dev` to `go build`
- **Never add `bun run build` to the `wails3 dev` pipeline** — it wastes time and is unused at runtime

## Run / Test

Automated checks are listed under Validation in `CLAUDE.md` (`go test ./...`, `bun run check`, etc.); tests must never call network services. Tray, multi-monitor, and live playback behavior still need manual testing on a Windows machine with multiple monitors. Verify by running the built binary directly:

```nu
.\bin\livepaper.exe 'C:\Wallpapers\test.jpg'
```

## Frameworks And Runtime

- Go `1.26.3`
- Wails `v3.0.0-beta.9`
- Astro `7.2.3`
- Win32 APIs via `golang.org/x/sys` and `syscall`
- `ffmpeg` / `ffprobe` / `mpv` for video processing and playback — bundled into `bin/` at build time; at runtime found beside `livepaper.exe` or on `PATH`
- Bun as the JavaScript runtime — **do not use `node` or `npm`**, always use `bun`

## Project Structure

- `cmd/livepaper/main.go` — CLI entrypoint; no args = tray mode, args = wallpaper apply mode
- `cmd/livepaper/tray.go` — Wails app bootstrap, tray, hidden window lifecycle
- `cmd/livepaper/service.go` — bridge ระหว่าง frontend กับ `internal/wallpaper`
- `internal/wallpaper/*.go` — monitor detection, image compose, video pipeline, Win32 desktop embedding
- `src/pages/index.astro` — tray UI หลักและ browser-side flow
- `src/components/*` / `src/styles/app.css` — UI pieces และ styling (Displays, Library, Settings)
- `public/scripts/*.js` — browser-side modules ของ tray UI (displays, gallery, local library history, settings, hotkeys)
- `public/wails/runtime.js` — runtime bridge ที่ frontend ใช้เรียก Go service
- `cmd/livepaper/assets_dev.go` / `assets_prod.go` — dev proxy กับ prod embedded assets
- `scripts/generate-icons.js` — icon generation (run with `bun`, not `node`)
- `scripts/bundle-media-tools.ps1` — build-time bundler สำหรับ `ffmpeg` / `ffprobe` / `mpv` ลง `bin/`
- `scripts/installer.bat` / `installer/livepaper.nsi` — local installer build และ NSIS script (offline payload)
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
2. `index.astro` โหลด monitors, version, dependency status (`CheckDependencies` หา tools ข้าง `livepaper.exe` หรือใน `PATH`), และ restore state เก่า
3. frontend เรียก `AppService` ผ่าน `runtime.js` เพื่อ browse file, generate thumbnail, และ preprocess video
4. เมื่อกด Apply, frontend ส่ง assignments ไป `ApplyWallpapers()`
5. `service.go` compose canvas, set static wallpaper, แล้ว start video wallpapers แบบ background goroutine

### Asset delivery

- dev: `assets_dev.go` proxy ไป Astro dev server `http://localhost:9245`
- prod: `assets_prod.go` serve embedded `dist/`

## Key Constraints

- Windows-only
- final wallpaper is always one composite JPEG in `Span` mode
- monitor slots without assigned media render black
- offline-only: the shipped app and installer make no network calls — no login/SSO, billing, telemetry, online galleries, external links, or in-app/install-time downloads; media tools are bundled at build time (dev mode's proxy to the local Astro server is the only HTTP use)
- manual testing is still required for tray, multi-monitor, file dialogs, and live playback
- live wallpaper internals are sensitive on Windows 11 24H2; read `docs/wallpaper-internals.md#known-mistakes` before touching `desktop.go` or `video.go`
