# Project Structure

## Overview

`go-livepaper` เป็น Windows desktop app ที่รวม Go app หลัก, Wails runtime, และ Astro frontend ไว้ใน repo เดียว

## Folder Structure

```text
.
|-- cmd/livepaper/
|   |-- main.go
|   |-- tray.go
|   |-- service.go
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
|   |-- wails/runtime.js
|   `-- icon*.png|ico|svg
|-- scripts/
|   |-- generate-icons.js
|   `-- install-deps.ps1
|-- build/config.yml
`-- docs/
```

## What Each Area Owns

- `cmd/livepaper/` คือ app shell: argument parsing, tray boot, service bridge, asset serving
- `internal/wallpaper/` คือ core wallpaper engine
- `src/` กับ `public/` คือ tray UI และ runtime bridge
- `scripts/` คือ helper scripts สำหรับ icon/dependency setup
- `build/config.yml` คือ Wails build/dev entry
- `docs/` คือเอกสารอ้างอิงเชิงลึก

## Frameworks And Runtime

- Go `1.26.3`
- Wails `v3.0.0-alpha.96`
- Astro `6.3.8`
- Win32 API ผ่าน `golang.org/x/sys` และ `syscall`
- `ffmpeg` / `ffprobe` สำหรับ thumbnail, frame extraction, และ video preprocessing
- `mpv` สำหรับ embedded video playback ผ่าน `--wid`
- Bun เป็น JavaScript runtime มาตรฐานของ repo

## Runtime Modes

- CLI mode: `main.go` parse args, compose wallpaper, start video wallpaper ถ้าจำเป็น
- Tray mode: `tray.go` เปิด Wails system tray app, frontend คุยกับ `service.go`
- Asset loading: dev ใช้ proxy จาก `assets_dev.go`, prod ใช้ embedded files จาก `assets_prod.go`

## System Flow Summary

- startup: `main.go` เลือก `--clean`, CLI mode, หรือ tray mode
- image apply: `GetMonitors()` -> `LoadAndResizeImage()` -> draw canvas -> `SaveImageAs()` -> `SetWallpaper()`
- video apply: `PreprocessVideo()` -> `ExtractVideoFrame()` -> set fallback JPEG -> `RunVideoWallpapers()`
- tray restore: frontend โหลด state จาก `localStorage`, validate files, แล้ว `ApplyWallpapers()` ซ้ำ

## Related Docs

- Deep wallpaper internals: `docs/wallpaper-internals.md`
- Human-facing usage and install guide: `README.md`
