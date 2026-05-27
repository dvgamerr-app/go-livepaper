@echo off
setlocal enabledelayedexpansion

set "ROOT_DIR=%~dp0.."
pushd "%ROOT_DIR%" || exit /b 1

:: ── Version ────────────────────────────────────────────────────────────────────
for /f "tokens=*" %%v in ('git describe --tags --abbrev^=0 2^>nul') do set "GIT_TAG=%%v"
if not defined GIT_TAG set "GIT_TAG=v0.0.0"

set "APP_VERSION=%GIT_TAG:v=%"
set "APP_FILE_VERSION=%APP_VERSION%"

echo.
echo [livepaper installer] v%APP_VERSION%
echo.

:: ── Icons ──────────────────────────────────────────────────────────────────────
echo ^> bun scripts/generate-icons.js
bun scripts/generate-icons.js || goto :fail

:: ── Frontend ───────────────────────────────────────────────────────────────────
echo ^> bun run build
bun run build || goto :fail

:: ── Windows resource ───────────────────────────────────────────────────────────
echo ^> go run rsrc -ico public/icon.ico
go run github.com/akavel/rsrc@latest -ico public/icon.ico -o cmd/livepaper/rsrc_windows_amd64.syso || goto :fail

:: ── Go binary ──────────────────────────────────────────────────────────────────
if not exist bin mkdir bin
echo ^> go build -o bin/livepaper.exe
go build -ldflags "-H windowsgui -X main.VERSION=%APP_VERSION%" -o bin/livepaper.exe ./cmd/livepaper/ || goto :fail

:: ── NSIS installer ─────────────────────────────────────────────────────────────
echo.
echo ^> makensis /DAPP_VERSION=%APP_VERSION% /DAPP_FILE_VERSION=%APP_FILE_VERSION% installer\livepaper.nsi
makensis /DAPP_VERSION=%APP_VERSION% /DAPP_FILE_VERSION=%APP_FILE_VERSION% installer\livepaper.nsi || goto :fail

echo.
echo Installer complete: bin\livepaper-setup-%APP_VERSION%.exe
popd
exit /b 0

:fail
set "EXIT_CODE=%ERRORLEVEL%"
popd
exit /b %EXIT_CODE%
