# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Agent instructions

**Every time you try something and it is wrong, record it immediately in the "Known mistakes" section below with a one-line explanation of why it failed.** Do not attempt the same approach twice. Read "Known mistakes" at the start of any session involving `desktop.go` or `video.go` before writing any code.

## Build

```nu
^go build -o livepaper.exe ./cmd/livepaper
```

With version from `VERSION` file:

```nu
let version = (open VERSION | str trim)
^go build -ldflags $"-X main.VERSION=($version)" -o livepaper.exe ./cmd/livepaper
```

## Run / Test

There is no test suite. Manual testing requires a Windows machine with multiple monitors. Verify by running the built binary directly:

```nu
.\livepaper.exe 'C:\Wallpapers\test.jpg'
```

## Architecture

All source lives in `cmd/livepaper/` as a single `main` package. The flow is:

1. **main.go** — CLI parsing (`go-arg`), argument validation, orchestration. `selectMonitors` maps `-m` flags (1-based) to `MonitorInfo` structs. If the first positional argument is a video file, jumps to `runVideoWallpaper` immediately (skips image compositing).
2. **monitor.go** — Calls `EnumDisplayMonitors` / `GetMonitorInfoW` via `user32.dll`. `getCanvas` normalises all monitor coordinates to a 0-based origin **and returns `vdMinX, vdMinY`** (the raw virtual-desktop origin) so callers can un-normalise back to real Windows screen coordinates. Primary monitor is always at screen `(0,0)`.
3. **wallpaper.go** — Image pipeline: `loadAndResizeImage` → decode → read EXIF orientation → `resizeImageToFill` (fill/crop to exact monitor size) → `draw.Draw` onto the main canvas. `saveImageAs` writes a timestamped JPEG to `%TEMP%\livepaper\`.
4. **orientation.go** — Reads EXIF orientation tag from image file and returns an integer used by `applyOrientation` (in the same file) to rotate/flip the `image.Image`.
5. **win64_sys.go** — Two Windows API calls: `setWallpaperStyle` writes `WallpaperStyle` and `TileWallpaper` to `HKCU\Control Panel\Desktop`, then `setWallpaper` calls `SystemParametersInfoW(SPI_SETDESKWALLPAPER)` to apply the image.
6. **desktop.go** — Win32 window management for live wallpaper embedding (see below).
7. **video.go** — ffmpeg-based video decode + GDI render loop. `decodeVideoFrames` runs `ffmpeg -stream_loop -1 -hwaccel auto` piping raw BGRA frames. `renderFrames` blits at 30 fps via `GetDC`/`StretchDIBits`/`ReleaseDC`.

### Key design constraints

- **Windows-only**: uses `user32.dll`, `windows/registry`, and `syscall.UTF16PtrFromString` directly — no platform abstraction.
- **Single composite image**: always produces one full-desktop JPEG in `Span` style (value `22`). Per-monitor wallpaper APIs are not used.
- Monitors with no assigned wallpaper render as black on the canvas.
- Output quality is JPEG 90.
- Video mode only targets the primary monitor; multi-monitor video is not implemented.

## Live Wallpaper Embedding (desktop.go)

### Progman / WorkerW trick

Send undocumented message `0x052C` to `Progman` to trigger desktop background layer creation. **Always send `(0, 0)` parameters** — `(0xD, 0x1)` hides desktop icons on Windows 11 24H2 by moving SHELLDLL_DefView.

### Windows 11 24H2 vs Windows 10

On **Windows 10**, after `0x052C`:
- A full-size background `WorkerW` is created behind Progman
- `SHELLDLL_DefView` (icons) lives inside another `WorkerW` above it
- Embed video window as `SetParent` child of the background WorkerW → **Case A**

On **Windows 11 24H2**, the architecture changed completely:
- `0x052C` only creates tiny `WorkerW` windows (~136×39 px) — no full-size background layer
- `SHELLDLL_DefView` is a **direct child of Progman** (not inside a WorkerW)
- Progman has `WS_EX_NOREDIRECTIONBITMAP` set → DWM composites directly, bypassing GDI redirection
- `HWND_BOTTOM` cannot go below Progman — the shell locks Progman at the absolute z-order bottom

Detection: check `GetWindowLongPtr(hProgman, GWL_EXSTYLE) & WS_EX_NOREDIRECTIONBITMAP`.

### Case A — WorkerW available (Windows 10 / some Windows 11)

1. Find `WorkerW` containing `SHELLDLL_DefView` → that is the icon layer (`iconWW`)
2. The `WorkerW` enumerated after `iconWW` → background layer (`bgWW`), validate size ≥ `w/2 × h/2`
3. `SetParent(hwnd, bgWW)` cross-process (create window as `WS_POPUP` first; `CreateWindowExW` rejects `WS_CHILD` cross-process)
4. Switch `GWL_STYLE`: clear `WS_POPUP`, set `WS_CHILD`
5. Set `GWL_EXSTYLE = WS_EX_TOOLWINDOW`
6. `SetWindowPos` with `SWP_FRAMECHANGED` to apply style
7. Convert screen coords → WorkerW client coords via `GetWindowRect(bgWW)`
8. `SetWindowPos(hwnd, HWND_BOTTOM, clientX, clientY, w, h)`

### Case B — No full-size WorkerW (Windows 11 24H2)

Technique from Lively Wallpaper v2.2.0.0. **Order is critical** — see Known mistakes for what breaks at each step.

1. `CreateWindowExW(WS_EX_LAYERED, ..., WS_POPUP, screenX, screenY, w, h, ...)` — **must pass `WS_EX_LAYERED` at creation**, not set it afterwards
2. `SetParent(hwnd, hProgman)` — reparent cross-process
3. `SetWindowLongPtr(GWL_STYLE, WS_CHILD)` — clear `WS_POPUP`, set `WS_CHILD`
4. `SetWindowPos(SWP_FRAMECHANGED | SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE)` — notify Windows of style change; **this clears `WS_EX_LAYERED`**
5. `SetWindowLongPtr(GWL_EXSTYLE, WS_EX_LAYERED)` — **re-set after `SWP_FRAMECHANGED` cleared it**
6. `SetLayeredWindowAttributes(hwnd, 0, 255, LWA_ALPHA)` — fully opaque; must come **after** step 5 or it fails silently
7. Convert screen coords → Progman client coords: `clientX = screenX - progmanRect.left`, `clientY = screenY - progmanRect.top`
8. `hShellView = FindWindowEx(hProgman, 0, "SHELLDLL_DefView", 0)`
9. `SetWindowPos(hwnd, hShellView, clientX, clientY, w, h, SWP_NOACTIVATE)` — inserts just below icon layer; fall back to `HWND_BOTTOM` if no SHELLDLL_DefView
10. `ShowWindow(hwnd, SW_SHOW)`

### Coordinate system

`getCanvas` normalises all monitor positions to a `(0,0)` origin but the raw Windows screen keeps the primary monitor at `(0,0)` always. Un-normalise: `rawX = normalised.x + vdMinX`. On a machine where Monitor 2 (portrait) is physically above Monitor 1 (primary), `vdMinY` is negative (e.g. `-774`).

### Win32 constraints

- `CreateWindowExW` **rejects `WS_CHILD`** when the parent belongs to another process. Always create `WS_POPUP` then `SetParent` + style swap.
- `runtime.LockOSThread()` must be called in the same goroutine that creates the window and runs the message loop.
- `WM_SYSCOMMAND / SC_MINIMIZE` should be blocked in the wndProc to prevent Win+D from minimising; however, windows embedded in the shell layer (Case A or Case B) are not minimised by the shell and the block may be removed once embedding works correctly.

## Known mistakes

Read this before touching `desktop.go` or `video.go`. Do not repeat any of these.

### 0x052C parameters `(0xD, 0x1)` hide desktop icons on Windows 11 24H2
Sending `SendMessageTimeout(hProgman, 0x052C, 0xD, 0x1, ...)` moves SHELLDLL_DefView (desktop icons) and makes them invisible on Windows 11 24H2. Always use `(0, 0)` for both sends.

### `HWND_BOTTOM` cannot go below Progman
`SetWindowPos(hwnd, HWND_BOTTOM)` on a top-level window ends up **above** Progman, not below it. The shell locks Progman at the absolute z-order bottom. Confirmed by log: `GetWindow(Progman, GW_HWNDNEXT) = 0` (Progman is last) yet our window's next-below was Progman after HWND_BOTTOM.

### `SetWindowPos(hwnd, hProgman)` as insertAfter also places video above icons
Using `hProgman` as `hWndInsertAfter` also puts our window above Progman's icon layer when Progman is at the bottom. Does not achieve "below icons" for top-level windows.

### `SWP_FRAMECHANGED` always clears `WS_EX_LAYERED`
`SetWindowPos(SWP_FRAMECHANGED)` clears `WS_EX_LAYERED` every time, regardless of whether it was set at `CreateWindowExW` or via `SetWindowLongPtr` before the call. Confirmed: window created with `dwExStyle=WS_EX_LAYERED` shows `exStyle=0x8000` (wrong) after `SWP_FRAMECHANGED`. **Fix: call `SetWindowLongPtr(GWL_EXSTYLE, WS_EX_LAYERED)` and `SetLayeredWindowAttributes` again immediately AFTER the `SWP_FRAMECHANGED` call.**

### Embedding as Progman child without `WS_EX_LAYERED` → invisible
On Windows 11 24H2, Progman has `WS_EX_NOREDIRECTIONBITMAP` — DWM composites it directly. Child windows of Progman that do not have `WS_EX_LAYERED` are not included in the DWM composition tree and are simply invisible, even though `GetWindowRect` shows correct position and style shows `WS_CHILD|WS_VISIBLE`.

### `findBgWorkerW` original algorithm picked up tiny 136×39 utility WorkerW
The original algorithm searched for the first WorkerW WITHOUT SHELLDLL_DefView as a child. On this machine that found a 136×39 WorkerW at Monitor 2's position, leading to off-screen client coordinates `(-2560, 774)`. Minimum size check (`>= w/2 × h/2`) is required.

### `CreateWindowExW` rejects `WS_CHILD` when parent is in another process
Trying to create with `WS_CHILD` and `hParent=hProgman` (explorer.exe) returns Access Denied. Must create as `WS_POPUP` with no parent, then `SetParent` cross-process, then swap `GWL_STYLE` from `WS_POPUP` to `WS_CHILD`.

### Using normalised monitor coordinates for window positioning
`getCanvas` normalises all monitor coords to a (0,0) origin. Using these directly for Win32 window positioning places the window at wrong screen locations. Must un-normalise: `rawX = normalised.x + vdMinX`, `rawY = normalised.y + vdMinY`.

### `WS_EX_TOOLWINDOW` on Progman child makes window invisible
Setting `GWL_EXSTYLE = WS_EX_TOOLWINDOW` on a window embedded as a Progman child causes it to disappear on Windows 11 24H2. Use only `WS_EX_LAYERED` as the exstyle for Case B child windows.

### `SetLayeredWindowAttributes` called before `SetWindowLongPtr(GWL_EXSTYLE, WS_EX_LAYERED)` fails silently
`SetLayeredWindowAttributes` requires `WS_EX_LAYERED` to already be set on the window. Calling it before setting the exstyle does nothing — window remains invisible. Correct order: set `GWL_EXSTYLE = WS_EX_LAYERED` first, then call `SetLayeredWindowAttributes`.

### Top-level window approach for Case B — video visible but always above icons
Creating a top-level `WS_POPUP` window (not embedded in any shell window) makes video visible but always renders above icons regardless of z-order manipulation. Cannot achieve "behind icons" with a top-level window on Windows 11 24H2.
