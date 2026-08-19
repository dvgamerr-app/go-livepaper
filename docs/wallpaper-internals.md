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
4. `GetThumbnail` คืน preview ให้ UI และ cache JPEG/GIF ไว้ที่ `<exe-dir>\\data\\thumbnail`
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

### Video / ffmpeg / mpv

- cache-hit check ของ `PreprocessVideo`/`preprocessGIF` ที่เช็คแค่ `os.Stat(out)` (ว่ามีไฟล์) ไม่พอ: encode ที่ถูก kill/crash จะทิ้งไฟล์ `.mp4` ที่ truncated (ไม่มี moov atom) หรือ 0 byte ไว้ใน `%TEMP%\livepaper` แล้วถูก reuse → ffmpeg frame extract ออก `exit status 0xfffffffe` และ mpv ออก `exit status 2` บนไฟล์เดียวกัน วิธีแก้: validate ด้วย `IsPlayableVideo()` (ffprobe duration > 0 + size > 0) ก่อน reuse, ถ้าไม่ผ่านให้ลบแล้ว re-encode
- mpv exit codes: `1` = init ล้มเหลว/option ผิด, `2` = เล่นไฟล์ไม่ได้ (corrupt/unsupported/missing) — **ไม่ใช่** "bad arguments" ตามที่ comment เก่าเขียนไว้ ทั้งสองโค้ดเป็น permanent สำหรับ file+args เดิม จึงไม่ควร retry (จะ spin)
- `--no-terminal` ทำให้ mpv ไม่พ่นอะไรลง stderr เลย ดังนั้นถ้า mpv loop ออก exit 2 จะไม่เห็นเหตุผล — ต้อง validate ไฟล์ด้วย ffprobe ก่อน spawn แทนที่จะหวังพึ่ง stderr
- `cmd.Stderr = io.Discard` ใน `ExtractVideoFrame` ทำให้ error เหลือแค่ exit status เปล่า ๆ; capture stderr แล้วแนบบรรทัดสุดท้ายของ ffmpeg (เช่น "moov atom not found") เข้า error
- บน Windows `cmd.Process.Kill()` ทำให้ process ออก exit code `1`; ใน `spawnMpv` ต้องเช็ค stop channel ก่อน classify exit ไม่งั้น teardown ปกติจะ log error หลอก
- `IsPlayableVideo` ต้องยอมให้ผ่าน (return true) เมื่อหา `ffprobe` ไม่เจอ (`exec.ErrNotFound`) มิฉะนั้น setup ที่ไม่มี ffprobe จะ reject cache ที่ดีทุกไฟล์

### Image / EXIF

- `Warning: Could not extract EXIF data: exif: failed to find exif intro marker` ที่เด้งทุกครั้งตอน apply **ไม่ใช่** error ของ `background.jpg` — มันมาจาก `getOrientation()` ที่อ่าน EXIF ของ **source image ที่ผู้ใช้เลือก** ใน `LoadAndResizeImage`. `background.jpg` (composite output ของ `SaveImageAs`) ไม่เคยถูกอ่าน EXIF กลับ ดังนั้นการเขียน EXIF ลง background.jpg ไม่ช่วยอะไร
- รูปส่วนใหญ่ (PNG, WebP, screenshot, JPEG ที่ generate เอง) ไม่มี EXIF block อยู่แล้ว → `exif.Decode` คืน error (`EOF` หรือ `failed to find exif intro marker`) เป็นเรื่องปกติ แปลว่า "ไม่ต้องหมุน". การอ่าน orientation เป็น best-effort + non-fatal (fallback = 1 เสมอ) จึงไม่ควร log warning ทุกครั้ง — string-match error message เปราะ (มีหลายข้อความ) ให้เงียบไปเลยบน decode error

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
- อย่าอ้าง `scripts/build-test-installer.bat` จาก `package.json` โดยสมมติว่าไฟล์ยังอยู่; working tree ปัจจุบันไม่มีไฟล์นี้ ให้ตรวจด้วย `rg --files scripts` ก่อนอ่านหรือเรียกใช้งาน
- ดาวน์โหลด `https://downloads.sourceforge.net/project/nsis/.../nsis-3.12.zip` ด้วย `Invoke-WebRequest` ตรง ๆ ใน environment นี้ได้หน้า HTML แทน ZIP ทำให้ `Expand-Archive` ล้มเหลว; อย่าใช้ URL รูปแบบนี้ซ้ำโดยไม่มีการ resolve redirect และตรวจชนิดไฟล์
- อย่า patch `SettingsView.astro` โดยเดาชื่อ `data-setting` จาก aria-label; working tree ใช้ `gpuAcceleration`, `reduceMotionOnFocus`, และ `restoreLastPlaylist` ซึ่งไม่ตรงกับชื่อย่อที่คาดไว้ ให้ inspect บรรทัดจริงก่อนแก้ toggle markup
- `bunx` ไม่อยู่ใน PATH ของ PowerShell session นี้; เรียก local frontend tools ด้วย `bun x <tool>` ตามข้อกำหนด Bun-only ของ repo
- อย่าใช้ `bun --check public/scripts/ui.js` เป็น standalone syntax check; browser-root import เช่น `/scripts/store.js` จะถูก Bun resolve เป็น filesystem path และล้มเหลว ให้ใช้ Astro production build เพื่อตรวจ integration ของ public scripts แทน
- บน PowerShell/Windows อย่าส่ง wildcard path เช่น `src/components/*.astro` ให้ `rg` โดยตรง เพราะ wildcard ไม่ถูก expand; ใช้ `rg -g '*.astro' <pattern> src/components` แทน
- บน PowerShell/Windows อย่าส่ง wildcard path เช่น `cmd/livepaper/*_test.go` หรือ `cmd/livepaper/*.go` ให้ `rg` โดยตรง เพราะ wildcard ไม่ถูก expand; ระบุ directory แล้วใช้ `-g '*_test.go'` / `-g '*.go'` แทน
- ตอนย้าย thumbnail cache อย่าตรวจเฉพาะ `GetThumbnail` และ animated hover preview; `makeUploadGIF` ก็เขียน `*_upload_thumb.gif` ลง cache เช่นกัน ต้องค้นทุกจุดที่ประกอบ filename ของ thumbnail ก่อนสรุป
- อย่าใส่ `*** Update File` ต่อท้าย hunk ของ `apply_patch` หลัง marker `@@` ที่ไม่มีเนื้อหา เพราะ parser จะมองว่าเป็นบรรทัดใน hunk และ reject ทั้ง patch; ปิด hunk ด้วย context จริงก่อนเริ่มไฟล์ถัดไป
- ตอนแทรก test ด้วย `apply_patch` อย่าใช้ fragment ของ comment เป็น anchor ถ้ามีหลายบรรทัดหรือ whitespace ที่อาจไม่ตรง; ใช้ชื่อฟังก์ชันถัดไปเป็น context ที่แน่นอนกว่า
- อย่ารวม production build-check กับ `Remove-Item` artifact ไว้ใน PowerShell command เดียวใน environment นี้ แม้ validate path แล้วก็อาจถูก policy ปฏิเสธทั้งคำสั่ง; แยก build verification ออกจาก cleanup
- แม้แยกคำสั่งแล้ว policy ของ environment นี้ยังปฏิเสธ `Remove-Item` สำหรับ temp build artifact ที่ระบุ path ชัดเจน; เมื่อ build-check สำเร็จให้รายงาน path และอย่าลองลบซ้ำด้วย shell อื่น
- เมื่อเช็กว่า Astro dev server ถูกปิดแล้วด้วย `Get-NetTCPConnection`, กรณีไม่มี listener อาจคืน exit code 1 และทำให้ parallel check ทั้งชุดดูเหมือน fail; จัดการกรณีว่างให้ exit 0 หรือรันแยก
- อย่าวาง `if (...) { ... } else { ... }` เป็น element ตรง ๆ ภายใน PowerShell array expression `@(...)`; PowerShell จะพยายามเรียก `if` เป็นคำสั่ง ให้ใช้ subexpression `$(if (...) { ... } else { ... })` หรือแยก statement ออกมาก่อน
- เมื่อ parser ใช้ named return แล้วสะสมค่าไประหว่างทาง อย่าคืนค่าที่สะสมพร้อม `ok=false`; กรณี `parseCombo` ที่มีแต่ modifier เคยคืน modifier flags ทั้งที่ parse ไม่สำเร็จ ให้ invalid path คืน `(0, 0, false)` เสมอ
- อย่าใช้ PowerShell here-string pipe เข้า `git apply --cached` เพื่อ stage เฉพาะบรรทัด Unicode ในไฟล์ที่ worktree มี diff อื่นค้างอยู่โดยคาดว่า context จะตรงเสมอ; ครั้งนี้ patch ใช้กับ index ไม่สำเร็จ ให้สร้าง blob จาก `HEAD` แล้วอัปเดต index แบบเจาะจงแทน
- ตอนลบ conflict markers ด้วย `apply_patch` อย่าอาศัย hunk ที่เริ่มกลาง conflict เพราะอาจลบ `=======`/`>>>>>>>` แต่เหลือ `<<<<<<<`; หลังทุกชุดต้องสแกนทั้ง repo ด้วย `rg '^(<<<<<<<|=======|>>>>>>>)'` ก่อน stage
- หลังเลือก conflict stages ด้วย `git checkout --ours/--theirs` อย่าคาดว่าไฟล์ที่ได้จะยังตรงกับ formatter ของ dependency version ใหม่; รอบนี้ `bun run format` พบ 17 ไฟล์ ให้รัน `bun run format:fix` หลัง resolve แล้วตรวจซ้ำ
- การวินิจฉัยว่า `%ProgramFiles(x86)%` ทำให้ batch แตกเป็น `'ist'` เป็นสมมติฐานผิด; การย้ายค่าไป delayed expansion จึงไม่แก้อาการ และอย่าแก้ parser จากข้อความ error โดยยังไม่มี command trace
- การย้าย NSIS download block ไป subroutine ช่วยลด nested parsing แต่ไม่ได้แก้ `'ist'` เพราะ failure เกิดก่อนถึง NSIS; ต้อง trace ลำดับคำสั่งจริงก่อน refactor control flow
- การตัด fallback `ProgramFiles(x86)` ออกเป็นอีกการแก้ที่ไม่ตรงสาเหตุ เพราะ error ยังเกิดเหมือนเดิม; path นี้ไม่ใช่ root cause
- การตัด fallback `ProgramFiles(x86)` ออกแล้วยังพบ `'ist'` เหมือนเดิม แปลว่า path นี้ไม่ใช่ root cause; อย่าแก้ batch parser จากการคาดเดาข้อความ error อย่างเดียว ให้เปิด command trace เพื่อดูบรรทัดที่ execute จริงก่อนเปลี่ยนโครงสร้างเพิ่ม
- `apply_patch` บน `scripts/installer.bat` ที่เดิมเป็น CRLF ทำให้ช่วงที่แก้กลายเป็น LF-only และไฟล์มี mixed line endings; `cmd.exe` กลืนบรรทัด `go build` ต่อกับต้น `if not exist` จนเหลือคำสั่ง `ist`. หลัง patch ไฟล์ `.bat` ต้อง normalize ทั้งไฟล์กลับเป็น CRLF ก่อนรัน
- อย่าเริ่ม `*** Update File` ถัดไปหลัง `@@` ว่างใน `apply_patch`; รอบนี้เผลอทำซ้ำข้อผิดพลาดเดิมจน patch หลายไฟล์ถูก reject ทั้งก้อน ให้ปิดไฟล์แรกด้วย hunk ที่มี context จริงหรือแยก patch

## Manual Verification

ใช้ flow นี้เมื่อแตะ `desktop.go` หรือ `video.go`:

```nu
let version = (open VERSION | str trim)
mkdir bin
^go build -ldflags $"-X main.VERSION=($version)" -o bin/livepaper.exe ./cmd/livepaper/
.\bin\livepaper.exe 'C:\Wallpapers\test.jpg'
```
