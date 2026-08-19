Use `AGENTS.md` as the primary repo prompt. Its build constraints and Known Mistakes workflow are mandatory.

## Project boundaries

- Windows-only Go 1.26.3 / Wails v3 application with an Astro frontend built by Bun.
- Production embeds `cmd/livepaper/dist`; development proxies the Astro server and must not build production assets first.
- DOM IDs in `src/components` are API contracts consumed by `public/scripts`. Preserve them or update every selector in the same change.
- Do not call Facebook/community APIs, SSO, billing, or storage services from automated tests.
- Read `docs/wallpaper-internals.md#known-mistakes` before changing `desktop.go` or `video.go`. This optimization pass intentionally did not touch those files.

## Validation

```powershell
bun run check
bun run format
bun run build
go test ./...
go vet ./...
go build -o "$env:TEMP\go-livepaper-check.exe" ./cmd/livepaper
git diff --check
```

Manual Windows/Wails verification is still required for tray interactions, multi-monitor layout, file dialogs, and live playback.

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

## Evidence

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
