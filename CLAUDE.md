Use `AGENTS.md` as the primary repo prompt. Its build constraints and Known Mistakes workflow are mandatory.

## Project boundaries

- Windows-only Go 1.26.6 / Wails v3 application with an Astro frontend built by Bun.
- Production embeds `cmd/livepaper/dist`; development proxies the Astro server and must not build production assets first.
- DOM IDs in `src/components` are API contracts consumed by `public/scripts`. Preserve them or update every selector in the same change.
- The shipped app and installer are offline-only: no runtime or install-time network calls, accounts, telemetry, online galleries, or external links. Automated tests must not call network services either.
- `ffmpeg`, `ffprobe`, and `mpv` are bundled into `bin/` at build time by `scripts/bundle-media-tools.ps1` (with licenses under `bin/licenses`) and are found beside `livepaper.exe` or on `PATH` at runtime. Never reintroduce an in-app or installer download step.
- Read `docs/wallpaper-internals.md#known-mistakes` before changing `desktop.go` or `video.go`. This optimization pass intentionally did not touch those files.

## Validation

```powershell
bun run check
bun run format
bun run build
go test ./...
go vet ./...
go build -tags production -o "$env:TEMP\go-livepaper-check.exe" ./cmd/livepaper
git diff --check
pwsh -NoProfile -Command '$e = $null; $null = [System.Management.Automation.Language.Parser]::ParseFile("$PWD\scripts\bundle-media-tools.ps1", [ref]$null, [ref]$e); $e'
```

Manual Windows/Wails verification is still required for tray interactions, multi-monitor layout, file dialogs, live playback, and the installer payload (bundled tools under `%LOCALAPPDATA%\Programs\livepaper\bin`).

## Optimization log

### 2026-08-02

This pass started from a heavily dirty working tree (43 tracked paths changed, 2,406 insertions and 4,635 deletions). All pre-existing work was preserved.

- Pin TypeScript to `^6.0.3`, the supported peer range of `@astrojs/check`; TypeScript 7 caused the checker to crash before diagnostics. Astro check now completes with zero diagnostics.
- Add `ModalShell.astro` for the shared overlay, dialog semantics, panel border/background, and shadow used by monitor, upload, delete, and sign-out dialogs. Modal IDs and JS behavior remain unchanged.
- Add `ViewHeader.astro` for the repeated Displays, Discover, Storage, and Library header structure.
- Replace 21 presentational elements with CSS pseudo-elements: monitor/grid decoration, gallery edge fades, sidebar icon wrappers/active dots, and eight toggle knobs.
- Consolidate repeated sidebar navigation and titlebar control utilities into component classes.
- Cache static footer/view DOM references in `ui.js` and scan wallpaper assignments once per refresh instead of calling `Object.values(...).some(...)` twice.
- Add explicit button types and dialog labels to shared modal flows to prevent accidental form submission and improve accessible names.
- Update README requirements, desktop UI capabilities, production asset order, development flow, validation commands, and architecture references.

### 2026-10-08

Offline conversion: the app is now a plain offline wallpaper setter. Every internet-dependent feature is removed instead of disabled.

- Scope of removal: SSO login/session/sign-out, billing, GitHub connections, Discover/community wallpapers, admin Storage upload/patch/delete, telemetry, `DownloadToTemp`, `OpenExternal` (and `github.com/pkg/browser`), and the in-app dependency installer (`InstallDependencies` and the DepWarn "Install" flow). Displays, Library (local recent history in IndexedDB), Settings General/Performance/Startup/Hotkeys/About, tray, hotkeys, power/focus watchers, `CheckDependencies`, thumbnail cache, and CLI apply mode stay.
- Replace the install-time `scripts/install-deps.ps1` (winget, user `PATH` edits, GitHub downloads on the user's machine) with the build-time `scripts/bundle-media-tools.ps1`. It downloads `ffmpeg.exe` + `ffprobe.exe` (BtbN/FFmpeg-Builds win64 GPL) and `mpv.exe` (shinchiro/mpv-winbuild-cmake x86_64) into `-OutputDir` (default `bin`), decides "already present" from the output directory only, verifies GitHub-published SHA-256 digests when available, copies upstream license files plus a source note into `bin/licenses`, cleans its temp directory, and exits non-zero when any tool is missing. `-Force` re-downloads.
- `scripts/installer.bat` and the release workflow run the bundler into `bin` before `makensis`; the workflow passes `GITHUB_TOKEN` for the GitHub API only.
- `installer/livepaper.nsi` no longer runs PowerShell or downloads anything. It packages an explicit payload (`livepaper.exe`, `ffmpeg.exe`, `ffprobe.exe`, `mpv.exe`, `licenses/`), so makensis fails when a tool is missing and stale `bin/scripts`, `bin/data`, or setup executables are never shipped. Upgrades delete the old `bin\scripts` directory and the legacy `bin\data` wallpaper cache; uninstall still removes the whole `bin` directory.
- Remove the tracked `bin/scripts/install-deps.ps1` copy and the admin upload guide `docs/upload-wallpaper.html`. Fix `.gitignore` so `/bin/` build output is ignored (the old `./bin` pattern matched nothing).
- Update README, AGENTS.md, and `docs/project-structure.md` for the offline-only app, bundled media tools, and GPL third-party notices.

#### Evidence (2026-10-08, build tooling)

- PowerShell parser check of `scripts/bundle-media-tools.ps1`: 0 errors in PowerShell 7 and Windows PowerShell 5.1.
- Bundler run against an output directory that already held the tools and source notes: exit 0 with no network access.
- `pwsh` bundler runs while `HTTP(S)_PROXY` pointed at a closed local port (default output directory, and an output directory missing only `licenses\mpv-SOURCE.txt`): printed the error, exited 1, and left no `livepaper-bundle-*` temp directory.
- The same proxy trick does not apply to Windows PowerShell 5.1, so one unintended real download ran into the ignored `bin/` (recorded in Known Mistakes). It exited 0, verified the GitHub SHA-256 digest of the ffmpeg zip, the mpv 7z, and `7zr.exe`, copied `ffmpeg-LICENSE.txt` (GPLv3), and warned that the mpv archive contains no license file. Payload: `ffmpeg.exe` and `ffprobe.exe` at about 169 MB each, and `mpv.exe` at about 121 MB.
- `scripts/installer.bat` keeps CRLF line endings (89 CRLF, 0 bare LF).
- A full `makensis` build was not run in this step.

## Evidence (2026-08-02)

Baseline frontend output: 608 elements, 65,282-byte HTML, 65,997-byte CSS, and 6,291-byte `ui.js`.

After refactoring: 587 elements, 61,401-byte HTML, 66,630-byte CSS, and 6,353-byte `ui.js`. The built HTML/stylesheet/`ui.js` total decreased by 3,186 bytes while removing 21 DOM elements.

Final verification:

- `bun install --frozen-lockfile`: passed with no lockfile changes.
- `bun run check`: passed for 53 files with 0 errors, warnings, or hints.
- `bun run format` and `bun run build`: passed; one static page was generated.
- `go test -count=1 ./...`, `go vet ./...`, and a temporary `go build`: passed.
- `git diff --check`: passed. Git reports only the existing working-tree LF-to-CRLF conversion warnings.
- A read-only repo-wide `gofmt -d` audit still reports pre-existing Go formatting and line-ending differences. This pass did not rewrite untouched dirty Go files.
- Interactive Wails tray behavior, Windows multi-monitor positioning, file dialogs, and live video playback still require manual verification on a suitable desktop setup.
