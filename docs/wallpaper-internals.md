# Wallpaper Internals

## Scope

เอกสารนี้รวมรายละเอียดเชิงลึกของระบบ wallpaper/video embedding ที่ย้ายออกจาก `AGENTS.md` เพื่อลด token แต่ยังเก็บไว้สำหรับงานที่แตะ `internal/wallpaper/`, `desktop.go`, หรือ `video.go`.

## Core Package Layout

### `internal/wallpaper/win32.go`

- ประกาศ `user32.dll`, `kernel32.dll`, และ proc variables ที่ใช้ร่วมกันทั้ง package

### `internal/wallpaper/monitor.go`

- `GetMonitors()` เรียก `EnumDisplayMonitors` และ `GetMonitorInfoW`
- `getCanvas()` normalize monitor coordinates ให้เริ่มที่ `(0,0)`
- คืน `vdMinX`, `vdMinY` มาด้วยเพื่อใช้ un-normalize กลับไป screen coordinates จริง

### `internal/wallpaper/image.go`

- `LoadAndResizeImage` decode ภาพ, อ่าน EXIF orientation, แล้ว resize/crop
- `SaveImageAs` เขียน JPEG ลง `%TEMP%\\livepaper\\`
- monitor ที่ไม่ได้ถูก assign ภาพจะยังคงเป็นพื้นดำจาก canvas หลัก

### `internal/wallpaper/orientation.go`

- อ่าน EXIF orientation แล้ว apply rotation/flip ให้ภาพก่อน compose

### `internal/wallpaper/sys.go`

- ตั้ง registry ที่ `HKCU\\Control Panel\\Desktop`
- ใช้ `SystemParametersInfoW(SPI_SETDESKWALLPAPER)` เพื่อ apply wallpaper
- style หลักคือ `Span` (`22`)

### `internal/wallpaper/desktop.go`

- จัดการ Win32 host window สำหรับ live wallpaper
- ดูแล desktop embedding strategy, message loop, และ parent/z-order logic

### `internal/wallpaper/video.go`

- preprocess video
- extract mid-frame สำหรับ static fallback
- เปิด ffmpeg หรือ mpv workflow สำหรับ live wallpaper playback

## Desktop Embedding Model

## Progman / WorkerW trigger

ส่ง message `0x052C` ไปที่ `Progman` เพื่อให้ shell สร้าง background layer.

ข้อกำหนดสำคัญ:

- ส่ง `(0, 0)` เท่านั้น
- อย่าใช้ `(0xD, 0x1)` เพราะบน Windows 11 24H2 จะทำให้ desktop icons หาย

## Windows 10 vs Windows 11 24H2

### Case A: WorkerW background layer ใช้งานได้

พบบ่อยใน Windows 10 และบางเครื่อง Windows 11:

1. หา `WorkerW` ที่มี `SHELLDLL_DefView` เป็นลูก เพื่อระบุ icon layer
2. ใช้ `WorkerW` ถัดไปที่มีขนาดใหญ่พอเป็น background layer
3. สร้าง window เป็น `WS_POPUP`
4. `SetParent` ไปยัง background `WorkerW`
5. สลับ style เป็น `WS_CHILD`
6. ใช้ `WS_EX_TOOLWINDOW`
7. apply `SWP_FRAMECHANGED`
8. แปลง screen coordinates เป็น client coordinates ของ `WorkerW`
9. จัด z-order ให้อยู่หลัง icon layer

### Case B: ไม่มี full-size WorkerW

พบบน Windows 11 24H2:

1. สร้าง window ด้วย `WS_EX_LAYERED | WS_POPUP`
2. `SetParent(hwnd, hProgman)`
3. เปลี่ยน style เป็น `WS_CHILD`
4. เรียก `SetWindowPos(... SWP_FRAMECHANGED ...)`
5. ตั้ง `GWL_EXSTYLE = WS_EX_LAYERED` ซ้ำอีกครั้ง เพราะขั้นก่อนหน้าล้าง flag นี้
6. เรียก `SetLayeredWindowAttributes`
7. แปลง screen coordinates เป็น Progman client coordinates
8. หา `SHELLDLL_DefView`
9. แทรกหน้าต่างไว้ใต้ icon layer ด้วย `SetWindowPos(hwnd, hShellView, ...)`
10. `ShowWindow(hwnd, SW_SHOW)`

## Coordinate Model

- canvas ภายใน app ใช้ normalized coordinates
- Windows desktop จริงยึด primary monitor เป็น `(0,0)` เสมอ
- เวลาจะสร้างหรือย้าย Win32 window ต้อง un-normalize:

```text
rawX = normalizedX + vdMinX
rawY = normalizedY + vdMinY
```

## Static Wallpaper Pipeline

1. `GetMonitors()` เก็บ monitor layout และ canvas size
2. สร้าง black canvas ขนาด desktop รวม
3. โหลดรูปต่อ monitor และ resize ให้พอดีกรอบ
4. draw ลงตำแหน่ง normalized ของ monitor นั้น
5. เขียนออกเป็น JPEG quality 90
6. ตั้ง wallpaper style เป็น `Span`
7. call `SetWallpaper`

## Video Wallpaper Pipeline

1. ตรวจว่าไฟล์เป็น video หรือไม่
2. encode/crop ใหม่ให้ตรงกับ monitor target
3. extract frame กลางวิดีโอเพื่อใช้เป็น static background
4. apply composite JPEG ก่อนเริ่ม live playback
5. สร้าง desktop host window
6. embed player หรือ frame output ลง shell layer
7. loop จน process ถูก stop

## Tray App Flow

1. Wails app เริ่มจาก `cmd/livepaper/tray.go`
2. frontend เรียก `AppService` methods ผ่าน runtime bridge
3. `BrowseFile` เปิด native file picker
4. `GetThumbnail` คืน preview ให้ UI
5. `PreprocessVideo` emit progress event `video:progress`
6. `ApplyWallpapers` compose ภาพ, apply wallpaper, แล้วค่อย start video goroutine
7. frontend เก็บ state ไว้ใน `localStorage` เพื่อ restore session

## Constraints

- Windows-only
- ใช้ composite JPEG เดียวเสมอ ไม่ใช้ Windows per-monitor wallpaper API
- monitor ที่ไม่มี assignment จะเป็นสีดำ
- video multi-monitor แบบหลาย stream ยังไม่รองรับครบ
- manual verification ต้องรันบนเครื่อง Windows จริง โดยเฉพาะกรณีหลายจอ

## Known Mistakes

อ่านก่อนแก้ `desktop.go` หรือ `video.go` และบันทึก mistake ใหม่ไว้ที่นี่ทันทีเมื่อทดลองอะไรแล้วผิดพลาด

### Desktop / Win32

- `0x052C` parameters `(0xD, 0x1)` ทำให้ icons หายบน Windows 11 24H2
- `HWND_BOTTOM` ไม่สามารถลงไปต่ำกว่า Progman ได้
- ใช้ `hProgman` เป็น `insertAfter` ก็ยังวาง video ไว้เหนือ icons
- `SWP_FRAMECHANGED` จะล้าง `WS_EX_LAYERED`
- child ของ Progman ที่ไม่มี `WS_EX_LAYERED` จะมองไม่เห็นบน 24H2
- algorithm หา background `WorkerW` แบบเดิมอาจไปเจอ utility window จิ๋ว 136x39
- `CreateWindowExW` สร้าง `WS_CHILD` ข้าม process โดยตรงไม่ได้
- ใช้ normalized monitor coordinates ไปวาง Win32 window ตรง ๆ จะเพี้ยน
- `WS_EX_TOOLWINDOW` บน Progman child ทำให้ window หาย
- เรียก `SetLayeredWindowAttributes` ก่อนตั้ง `WS_EX_LAYERED` จะ fail แบบเงียบ
- top-level window สำหรับ Case B เห็นวิดีโอจริง แต่จะอยู่เหนือ icons เสมอ

### Wails / App

- `app.NewSystemTray()` ไม่มีใน Wails v3; ต้องใช้ `app.SystemTray.New()`
- `application.OpenFileDialog()` ไม่มี; ต้องใช้ `app.Dialog.OpenFile()`
- `wails3 dev` จะรอ frontend dev server ถ้าไม่ได้ build แบบ production tag
- restore state ที่ชี้ไป cached video ที่ถูกลบแล้วจะทำให้ monitor block ติดสถานะ encoding ถาวร

### Local Agent Environment

- ใช้ PowerShell-native shell flow ใน repo นี้ทำให้เจอ quoting/error ง่ายโดยไม่จำเป็น
- เรียก `nu.exe` หรือ `pwsh.exe` โดยไม่ quote path จะพังที่ `C:\\Program`
- ตั้ง `exec_command.shell` เป็น `nu.exe` ตรง ๆ ใน environment นี้ไม่ expose Nu builtins
- `rtk bun --version` เคย fail ด้วย `Access is denied`, ดังนั้นอย่าคาดหวังว่า `rtk` จะ spawn `bun` ได้เสมอ
- ใช้ Nushell `enumerate` กับ `.index` / `.item` shorthand ใน environment นี้ล้มเหลว; ใช้ `$row.index` และ `$row.item` แทน
- ส่ง Nushell `$row...` ผ่าน PowerShell double quotes จะโดน PowerShell strip ตัวแปรก่อนถึง Nushell
- ส่ง regex ให้ `rg` ผ่าน nested double quotes ของ PowerShell/Nushell ทำให้ pattern พัง; ใช้ single-quoted pattern ใน `nu -c`
- ซ่อน console ของ child process บน Windows ด้วย `syscall.CREATE_NO_WINDOW` ตรง ๆ ใน Go 1.26.3 ของ environment นี้ compile ไม่ผ่าน (`undefined: syscall.CREATE_NO_WINDOW`); ถ้าต้องใช้ flag นี้ให้ประกาศค่า Win32 เองแทน
- `bun run` บน Windows ที่ชี้ script เป็น `scripts\\build-test.bat` ตรง ๆ เคยถูก parse พังเป็น `scriptsbuild-test.bat`; เรียก batch ผ่าน `cmd /c` แทน
- `bun run` บน Windows ที่ใช้ `cmd /c scripts\\build-test.bat` ยังกลืน backslash จนกลายเป็น `scriptsbuild-test.bat`; ถ้าจะเรียก batch ผ่าน `package.json` ให้ใช้ forward slashes
- `bun run` บน Windows ที่ใช้ `cmd /c scripts/build-test.bat` จะโดน `cmd` มอง `/` เป็น option ของ path; ถ้าจะเรียก `.bat` ผ่าน `cmd /c` ให้ส่ง path แบบ quoted command
- อัดคำสั่ง `nu -c "^git diff ..."` ที่ quote ซ้อนหนัก ๆ เข้า `multi_tool_use.parallel` อาจล้มตั้งแต่ launcher setup; รันคำสั่ง inspection แบบเดี่ยวแทน
- `rtk go build -o livepaper-test.exe ./cmd/livepaper` ใน session นี้ compile ผ่านแต่ไม่ควรสมมติว่า artifact จะอยู่ที่ `livepaper-test.exe`; อย่าใช้ path นี้เป็นขั้นล้างไฟล์ต่อ

## Manual Verification

ใช้ flow นี้เมื่อแตะ `desktop.go` หรือ `video.go`:

```nu
let version = (open VERSION | str trim)
mkdir bin
^go build -ldflags $"-X main.VERSION=($version)" -o bin/livepaper.exe ./cmd/livepaper/
.\bin\livepaper.exe 'C:\Wallpapers\test.jpg'
```
