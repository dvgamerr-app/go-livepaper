# Project Structure

## Overview

`go-livepaper` เป็น Windows desktop app แบบ offline ล้วน ที่รวม Go app หลัก, Wails runtime, และ Astro frontend ไว้ใน repo เดียว ตัว app และ installer ไม่เรียก network เลย; `ffmpeg` / `ffprobe` / `mpv` ถูก bundle มากับ installer ตอน build

## Folder Structure

```text
.
|-- cmd/livepaper/
|   |-- main.go
|   |-- tray.go
|   |-- service.go
|   |-- settings.go
|   |-- hotkeys_windows.go
|   |-- watchers_windows.go
|   |-- assets_dev.go
|   `-- assets_prod.go
|-- internal/wallpaper/
|   |-- monitor.go
|   |-- image.go
|   |-- sys.go
|   |-- desktop.go
|   `-- video.go
|-- src/
|   |-- pages/index.astro
|   |-- components/
|   `-- styles/app.css
|-- public/
|   |-- scripts/*.js
|   |-- wails/runtime.js
|   `-- icon*.png|ico|svg
|-- scripts/
|   |-- generate-icons.js
|   |-- bundle-media-tools.ps1
|   `-- installer.bat
|-- installer/livepaper.nsi
|-- build/config.yml
`-- docs/
```

## What Each Area Owns

- `cmd/livepaper/` คือ app shell: argument parsing, tray boot, service bridge, settings, hotkeys, power/focus watchers, asset serving
- `internal/wallpaper/` คือ core wallpaper engine
- `src/` กับ `public/` คือ tray UI (Displays, Library, Settings) และ runtime bridge; Library คือประวัติ wallpaper ที่เคย apply เก็บใน IndexedDB บนเครื่อง
- `scripts/` คือ build helpers: icon generation, media tools bundler, และ local installer build
- `installer/livepaper.nsi` คือ NSIS script ที่ package `livepaper.exe`, media tools, และ `licenses/` แบบ offline
- `build/config.yml` คือ Wails build/dev entry
- `docs/` คือเอกสารอ้างอิงเชิงลึก

## Frameworks And Runtime

- Go `1.26.3`
- Wails `v3.0.0-beta.9`
- Astro `7.2.3`
- Win32 API ผ่าน `golang.org/x/sys` และ `syscall`
- `ffmpeg` / `ffprobe` สำหรับ thumbnail, frame extraction, และ video preprocessing
- `mpv` สำหรับ embedded video playback ผ่าน `--wid`
- Bun เป็น JavaScript runtime มาตรฐานของ repo

## Offline Policy

- app ที่ ship และ installer ห้ามเรียก network: ไม่มี login/SSO, billing, telemetry, online gallery, external links, หรือ in-app/install-time download
- `scripts/bundle-media-tools.ps1` ดาวน์โหลด `ffmpeg.exe` + `ffprobe.exe` (BtbN/FFmpeg-Builds win64 GPL) และ `mpv.exe` (shinchiro/mpv-winbuild-cmake x86_64) ลง `bin/` ตอน build เท่านั้น พร้อม copy license files และ source note ลง `bin/licenses`
- bundler นับว่ามี tool แล้วจากไฟล์ใน output directory เท่านั้น (ไม่ดู `PATH` ของเครื่อง build) และ exit non-zero ถ้าขาดไฟล์ใด
- `scripts/installer.bat` และ release workflow รัน bundler ก่อน `makensis`; NSIS script package ไฟล์แบบระบุชื่อชัดเจน จึงไม่ ship `bin/scripts`, `bin/data`, หรือ setup exe เก่า
- runtime: `main()` เรียก `addBundledToolsSearchPath()` ตอน startup (ทั้ง CLI และ tray) เพื่อเติม directory ของ `livepaper.exe` ไว้หน้า `PATH` ของ process แล้ว `exec.LookPath` จึงเจอ tools ที่ bundle มาก่อน; ถ้าไม่มีข้าง exe จะ fallback ไปใช้ `PATH` ของเครื่อง
- dev mode proxy ไป Astro dev server ที่ `localhost` เท่านั้น

## Runtime Modes

- CLI mode: `main.go` parse args, compose wallpaper, start video wallpaper ถ้าจำเป็น
- Tray mode: `tray.go` เปิด Wails system tray app, frontend คุยกับ `service.go`
- Asset loading: dev ใช้ proxy จาก `assets_dev.go`, prod ใช้ embedded files จาก `assets_prod.go`

## System Flow Summary

- startup: `main.go` เลือก `--clean`, CLI mode, หรือ tray mode
- image apply: `GetMonitors()` -> `LoadAndResizeImage()` -> draw canvas -> `SaveImageAs()` -> `SetWallpaper()`
- video apply: `PreprocessVideo()` -> `ExtractVideoFrame()` -> set fallback JPEG -> `RunVideoWallpapers()`
- tray restore: frontend โหลด state จาก `localStorage`, validate files, แล้ว `ApplyWallpapers()` ซ้ำ
- installer build: icons -> `bun run build` -> rsrc -> `go build` -> `bundle-media-tools.ps1 -OutputDir bin` -> `makensis`

## Related Docs

- Deep wallpaper internals: `docs/wallpaper-internals.md`
- Human-facing usage and install guide: `README.md`
