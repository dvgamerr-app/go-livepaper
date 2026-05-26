#Requires -Version 5.1
<#
.SYNOPSIS
    Silent installer for ffmpeg and mpv (livepaper dependencies).
.DESCRIPTION
    Downloads and installs ffmpeg and mpv from GitHub.
    Uses winget when available; falls back to direct GitHub download.
    Installs to %LOCALAPPDATA%\Programs\livepaper\bin and adds to user PATH.
#>

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\livepaper\bin",
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# ── Helpers ────────────────────────────────────────────────────────────────────

function Write-Step([string]$msg) { Write-Host "  >> $msg" -ForegroundColor Cyan }
function Write-Ok([string]$msg)   { Write-Host "  OK $msg" -ForegroundColor Green }
function Write-Warn([string]$msg) { Write-Host "  !! $msg" -ForegroundColor Yellow }

function Test-CommandExists([string]$cmd) {
    return $null -ne (Get-Command $cmd -ErrorAction SilentlyContinue)
}

function Add-ToUserPath([string]$dir) {
    $current = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($current -split ';' -contains $dir) { return }
    [Environment]::SetEnvironmentVariable('PATH', "$current;$dir", 'User')
    $env:PATH = "$env:PATH;$dir"
    Write-Ok "Added to PATH: $dir"
}

function Get-GitHubAssetUrl([string]$repo, [string]$assetRegex) {
    $api = "https://api.github.com/repos/$repo/releases/latest"
    $headers = @{ 'User-Agent' = 'livepaper-installer/1.0' }
    $release = Invoke-RestMethod -Uri $api -Headers $headers
    $asset = $release.assets | Where-Object { $_.name -match $assetRegex } | Select-Object -First 1
    if (-not $asset) {
        throw "No asset matching '$assetRegex' found in $repo latest release"
    }
    return $asset.browser_download_url
}

function Expand-ZipTo([string]$zipPath, [string]$destDir) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $destDir)
}

function Get-7za {
    # Download 7zr.exe (standalone 7-zip reducer) from ip7z/7zip GitHub releases.
    # 7zr.exe handles .7z archives only — sufficient for mpv extraction.
    $sevenZipDir = "$env:TEMP\livepaper-setup\7zip"
    $sevenZa = "$sevenZipDir\7zr.exe"
    if (Test-Path $sevenZa) { return $sevenZa }

    New-Item -ItemType Directory -Force -Path $sevenZipDir | Out-Null
    Write-Step "Downloading 7zr.exe bootstrap from ip7z/7zip..."

    $url = Get-GitHubAssetUrl 'ip7z/7zip' '^7zr\.exe$'
    Invoke-WebRequest -Uri $url -OutFile $sevenZa -UseBasicParsing
    return $sevenZa
}

function Expand-7zTo([string]$archivePath, [string]$destDir, [string]$sevenZa) {
    New-Item -ItemType Directory -Force -Path $destDir | Out-Null
    & $sevenZa x $archivePath "-o$destDir" -y | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "7z extraction failed (exit $LASTEXITCODE)" }
}

# ── Install ffmpeg ─────────────────────────────────────────────────────────────

function Install-FFmpeg {
    if (!$Force -and (Test-CommandExists 'ffmpeg')) {
        $ver = (ffmpeg -version 2>&1 | Select-Object -First 1)
        Write-Ok "ffmpeg already installed: $ver"
        return
    }

    Write-Host "`n[ffmpeg]" -ForegroundColor White

    # Try winget
    if (Test-CommandExists 'winget') {
        Write-Step "Installing via winget (Gyan.FFmpeg)..."
        $result = winget install --id Gyan.FFmpeg --silent --scope user `
            --accept-package-agreements --accept-source-agreements 2>&1
        if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq -1978335189) {
            Write-Ok "ffmpeg installed via winget"
            return
        }
        Write-Warn "winget failed (exit $LASTEXITCODE), falling back to GitHub..."
    }

    # GitHub fallback: BtbN/FFmpeg-Builds (zip)
    Write-Step "Fetching latest ffmpeg release from BtbN/FFmpeg-Builds..."
    $url = Get-GitHubAssetUrl 'BtbN/FFmpeg-Builds' 'ffmpeg-master-latest-win64-gpl\.zip$'
    $zipPath = "$env:TEMP\livepaper-setup\ffmpeg.zip"
    $extractDir = "$env:TEMP\livepaper-setup\ffmpeg-extract"

    New-Item -ItemType Directory -Force -Path (Split-Path $zipPath) | Out-Null
    Write-Step "Downloading ffmpeg: $url"
    Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

    Write-Step "Extracting..."
    Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Expand-ZipTo $zipPath $extractDir

    $binSrc = Get-ChildItem $extractDir -Directory | Select-Object -First 1
    $ffBin = Join-Path $binSrc.FullName 'bin'

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item "$ffBin\ffmpeg.exe"  $InstallDir -Force
    Copy-Item "$ffBin\ffprobe.exe" $InstallDir -Force
    Copy-Item "$ffBin\ffplay.exe"  $InstallDir -Force -ErrorAction SilentlyContinue

    Remove-Item $zipPath, $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Add-ToUserPath $InstallDir
    Write-Ok "ffmpeg installed to: $InstallDir"
}

# ── Install mpv ────────────────────────────────────────────────────────────────

function Install-Mpv {
    if (!$Force -and (Test-CommandExists 'mpv')) {
        $ver = (mpv --version 2>&1 | Select-Object -First 1)
        Write-Ok "mpv already installed: $ver"
        return
    }

    Write-Host "`n[mpv]" -ForegroundColor White

    # Try winget
    if (Test-CommandExists 'winget') {
        Write-Step "Installing via winget (mpv.mpv)..."
        $result = winget install --id mpv.mpv --silent --scope user `
            --accept-package-agreements --accept-source-agreements 2>&1
        if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq -1978335189) {
            Write-Ok "mpv installed via winget"
            return
        }
        Write-Warn "winget failed (exit $LASTEXITCODE), falling back to GitHub..."
    }

    # GitHub fallback: shinchiro/mpv-winbuild-cmake (7z)
    Write-Step "Fetching latest mpv release from shinchiro/mpv-winbuild-cmake..."
    $url = Get-GitHubAssetUrl 'shinchiro/mpv-winbuild-cmake' 'mpv-x86_64-\d{8}-git[^.]*\.7z$'
    $archivePath = "$env:TEMP\livepaper-setup\mpv.7z"
    $extractDir  = "$env:TEMP\livepaper-setup\mpv-extract"

    New-Item -ItemType Directory -Force -Path (Split-Path $archivePath) | Out-Null
    Write-Step "Downloading mpv: $url"
    Invoke-WebRequest -Uri $url -OutFile $archivePath -UseBasicParsing

    $sevenZa = Get-7za
    Write-Step "Extracting (7z)..."
    Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Expand-7zTo $archivePath $extractDir $sevenZa

    # mpv.exe is in the root of the extracted dir
    $mpvExe = Get-ChildItem $extractDir -Filter 'mpv.exe' -Recurse | Select-Object -First 1
    if (-not $mpvExe) { throw "mpv.exe not found in extracted archive" }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item $mpvExe.FullName $InstallDir -Force

    Remove-Item $archivePath, $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Add-ToUserPath $InstallDir
    Write-Ok "mpv installed to: $InstallDir"
}

# ── Main ───────────────────────────────────────────────────────────────────────

Write-Host "livepaper dependency installer" -ForegroundColor White
Write-Host "Install dir: $InstallDir"
Write-Host ""

try {
    New-Item -ItemType Directory -Force -Path "$env:TEMP\livepaper-setup" | Out-Null

    Install-FFmpeg
    Install-Mpv

    Write-Host ""
    Write-Host "Done. Restart your terminal for PATH changes to take effect." -ForegroundColor Green
} catch {
    Write-Host ""
    Write-Host "ERROR: $_" -ForegroundColor Red
    exit 1
} finally {
    Remove-Item "$env:TEMP\livepaper-setup" -Recurse -Force -ErrorAction SilentlyContinue
}
