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

if not exist bin\scripts mkdir bin\scripts || goto :fail
copy /Y scripts\install-deps.ps1 bin\scripts\install-deps.ps1 >nul || goto :fail

:: ── NSIS installer ─────────────────────────────────────────────────────────────
set "MAKENSIS="
for /f "delims=" %%m in ('where makensis 2^>nul') do if not defined MAKENSIS set "MAKENSIS=%%m"

if not defined MAKENSIS if exist "%ProgramFiles(x86)%\NSIS\makensis.exe" set "MAKENSIS=%ProgramFiles(x86)%\NSIS\makensis.exe"
if not defined MAKENSIS if exist "%ProgramFiles%\NSIS\makensis.exe" set "MAKENSIS=%ProgramFiles%\NSIS\makensis.exe"

if not defined MAKENSIS (
  set "NSIS_VERSION=3.12"
  set "NSIS_CACHE_ROOT=%TEMP%\livepaper-tools"
  set "NSIS_DIR=%TEMP%\livepaper-tools\nsis-3.12"
  set "NSIS_ARCHIVE=%TEMP%\livepaper-tools\nsis-3.12.zip"
  set "NSIS_DOWNLOAD_URL=https://sourceforge.net/projects/nsis/files/NSIS%%203/3.12/nsis-3.12.zip/download"

  echo.
  echo makensis was not found. Downloading NSIS !NSIS_VERSION!...
  if not exist "!NSIS_CACHE_ROOT!" mkdir "!NSIS_CACHE_ROOT!" || goto :fail

  where curl.exe >nul 2>nul || (
    echo ERROR: curl.exe is required to download NSIS.
    goto :fail
  )

  curl.exe --fail --location --retry 3 --silent --show-error ^
    --output "!NSIS_ARCHIVE!" "!NSIS_DOWNLOAD_URL!" || goto :fail
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command ^
    "$bytes = [System.IO.File]::ReadAllBytes($env:NSIS_ARCHIVE); if ($bytes.Length -lt 2 -or $bytes[0] -ne 0x50 -or $bytes[1] -ne 0x4b) { throw 'Downloaded NSIS archive is not a ZIP file.' }" || goto :fail
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command ^
    "Expand-Archive -LiteralPath $env:NSIS_ARCHIVE -DestinationPath $env:NSIS_CACHE_ROOT -Force" || goto :fail

  set "MAKENSIS=!NSIS_DIR!\makensis.exe"
)

if not exist "%MAKENSIS%" (
  echo ERROR: makensis.exe was not found after downloading NSIS.
  goto :fail
)

echo.
echo ^> "%MAKENSIS%" /DAPP_VERSION=%APP_VERSION% /DAPP_FILE_VERSION=%APP_FILE_VERSION% installer\livepaper.nsi
"%MAKENSIS%" /DAPP_VERSION=%APP_VERSION% /DAPP_FILE_VERSION=%APP_FILE_VERSION% installer\livepaper.nsi || goto :fail

echo.
echo Installer complete: bin\livepaper-setup-%APP_VERSION%.exe
popd
exit /b 0

:fail
set "EXIT_CODE=%ERRORLEVEL%"
popd
exit /b %EXIT_CODE%
