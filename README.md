# livepaper

`livepaper` คือ CLI สำหรับ Windows ที่รวมภาพหลายใบเป็น wallpaper ผืนเดียวแบบ `Span` แล้วตั้งเป็นพื้นหลังเดสก์ท็อปอัตโนมัติ เหมาะกับเครื่องที่มีหลายจอและต้องการกำหนดภาพคนละใบต่อจอจาก command line

## ความสามารถ

- ตรวจจับจำนวนจอ, ความละเอียด, และตำแหน่งของแต่ละจอจาก Windows อัตโนมัติ
- รวมภาพหลายใบเป็น wallpaper ผืนเดียวตาม layout จริงของ desktop
- กำหนดภาพแยกต่อจอได้ตามลำดับ หรือระบุจอเป้าหมายด้วย `-m/--monitor`
- ย่อและครอปภาพแบบ fill เพื่อให้เต็มแต่ละจอโดยรักษาสัดส่วนภาพ
- อ่านค่า EXIF orientation และหมุนภาพให้ถูกทิศก่อนนำไปใช้
- รองรับไฟล์ `jpg`, `jpeg`, และ `png`
- สร้างไฟล์ wallpaper ชั่วคราวเป็น JPEG ใน `%TEMP%\livepaper`
- ล้างไฟล์ชั่วคราวทั้งหมดได้ด้วย `--clean`

## ข้อกำหนด

- Windows
- Go ตามเวอร์ชันที่ระบุใน [go.mod](/e:/.dvgamerr/go-livepaper/go.mod)

โปรแกรมนี้ใช้ Windows API และ registry โดยตรง จึงไม่รองรับ Linux หรือ macOS

## Build

```nu
^go build -o livepaper.exe ./cmd/livepaper
```

ถ้าต้องการฝังเวอร์ชันจากไฟล์ `VERSION`:

```nu
let version = (open VERSION | str trim)
^go build -ldflags $"-X main.VERSION=($version)" -o livepaper.exe ./cmd/livepaper
```

## วิธีใช้

```text
livepaper.exe [--monitor MONITOR] [--clean] [WALLPAPER [WALLPAPER ...]]
```

ตัวเลือก:

- `-m`, `--monitor` ระบุหมายเลขจอแบบ 1-based ตามลำดับภาพที่ส่งเข้าไป เช่น `-m 2 -m 1`
- `-c`, `--clean` ล้างไฟล์ชั่วคราวใน `%TEMP%\livepaper`
- `--version` แสดงเวอร์ชัน
- `-h`, `--help` แสดง help

กติกาการจับคู่ภาพกับจอ:

- ถ้าไม่ระบุ `-m` โปรแกรมจะจับคู่ภาพตามลำดับจอที่ Windows enumerate ได้
- ถ้าระบุ `-m` จำนวนจอต้องเท่ากับจำนวนภาพ และภาพแต่ละใบจะถูกส่งไปยังจอที่ระบุไว้ตามลำดับ
- หมายเลขจอเริ่มที่ `1`

## ตัวอย่าง

ตั้ง wallpaper 1 ภาพสำหรับเครื่องจอเดียว:

```nu
.\livepaper.exe 'C:\Wallpapers\main.jpg'
```

ตั้ง 2 ภาพตามลำดับจอที่ตรวจพบ:

```nu
.\livepaper.exe 'C:\Wallpapers\left.jpg' 'C:\Wallpapers\right.png'
```

ตั้งภาพให้เฉพาะจอ 2 และจอ 3:

```nu
.\livepaper.exe -m 2 -m 3 'C:\Wallpapers\portrait.jpg' 'C:\Wallpapers\stats.png'
```

ล้างไฟล์ wallpaper ชั่วคราว:

```nu
.\livepaper.exe --clean
```

## พฤติกรรมการทำงาน

1. โปรแกรมอ่าน layout ของทุกจอจากระบบ
2. สร้าง canvas สีดำขนาดรวมของ desktop ทั้งหมด
3. โหลดภาพ, หมุนตาม EXIF, แล้วย่อแบบ fill ให้พอดีกับแต่ละจอ
4. วางภาพลงบน canvas ตามตำแหน่งจริงของจอ
5. บันทึกเป็น JPEG ชั่วคราว
6. ตั้งค่า wallpaper style เป็น `Span` และสั่งให้ Windows reload wallpaper

## ข้อจำกัดที่ควรรู้

- โปรแกรมสร้าง wallpaper เป็นภาพผืนเดียวเสมอ ไม่ได้ใช้ per-monitor wallpaper ของ Windows
- ถ้าระบุ `-m` เฉพาะบางจอ จอที่ไม่ได้ระบุจะเป็นพื้นสีดำในภาพสุดท้าย
- ถ้าส่งภาพมากกว่าจำนวนจอโดยไม่ระบุ `-m` ภาพส่วนเกินจะถูกข้าม
- output สุดท้ายถูกบันทึกเป็น JPEG ดังนั้นอาจมีการสูญเสียคุณภาพเล็กน้อยเมื่อเทียบกับ PNG ต้นฉบับ

## โครงสร้างไฟล์หลัก

- [cmd/livepaper/main.go](/e:/.dvgamerr/go-livepaper/cmd/livepaper/main.go) จัดการ CLI และ flow หลัก
- [cmd/livepaper/monitor.go](/e:/.dvgamerr/go-livepaper/cmd/livepaper/monitor.go) ตรวจจับจอและ layout
- [cmd/livepaper/wallpaper.go](/e:/.dvgamerr/go-livepaper/cmd/livepaper/wallpaper.go) resize, compose, และจัดการไฟล์ชั่วคราว
- [cmd/livepaper/orientation.go](/e:/.dvgamerr/go-livepaper/cmd/livepaper/orientation.go) อ่าน EXIF orientation
- [cmd/livepaper/win64_sys.go](/e:/.dvgamerr/go-livepaper/cmd/livepaper/win64_sys.go) ตั้งค่า wallpaper ผ่าน Windows API
