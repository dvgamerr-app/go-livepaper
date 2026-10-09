#Requires -Version 5.1
<#
.SYNOPSIS
    Build-time bundler for ffmpeg, ffprobe, and mpv (livepaper media tools).
.DESCRIPTION
    Downloads ffmpeg.exe + ffprobe.exe (BtbN/FFmpeg-Builds win64 GPL zip) and
    mpv.exe (shinchiro/mpv-winbuild-cmake x86_64 7z) into -OutputDir so the
    installer payload ships them beside livepaper.exe. Runs on the build
    machine only; the installed app and the installer never touch the network.

    A tool counts as present only when its exe and its licenses\<tool>-SOURCE.txt
    note already exist in -OutputDir. Tools on the build machine's PATH are
    ignored on purpose, otherwise the installer could silently ship without
    them. Upstream license files are copied into <OutputDir>\licenses.
    Exits non-zero if any exe or source note is missing at the end.
.PARAMETER OutputDir
    Destination directory (default: <repo>\bin).
.PARAMETER Force
    Re-download every tool even when it is already present in -OutputDir.
.PARAMETER RequireDigest
    Fail instead of warning when GitHub publishes no SHA-256 digest for an
    archive. Without it, the computed SHA-256 is still recorded in the
    <tool>-SOURCE.txt note so the shipped payload can be audited later.
#>

param(
    [string]$OutputDir = (Join-Path (Split-Path $PSScriptRoot -Parent) 'bin'),
    [switch]$Force,
    [switch]$RequireDigest
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

# Windows PowerShell 5.1 on older hosts may not offer TLS 1.2 to GitHub by default.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$OutputDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputDir)
$LicenseDir = Join-Path $OutputDir 'licenses'
$WorkDir = Join-Path ([System.IO.Path]::GetTempPath()) ("livepaper-bundle-" + [guid]::NewGuid().ToString('N'))
# A tool counts as bundled only together with its source note, so the installer
# never ships a binary without the matching license material.
$FFmpegFiles = @('ffmpeg.exe', 'ffprobe.exe', 'licenses\ffmpeg-SOURCE.txt')
$MpvFiles = @('mpv.exe', 'licenses\mpv-SOURCE.txt')

# ── Helpers ────────────────────────────────────────────────────────────────────

function Write-Step([string]$msg) { Write-Host "  >> $msg" -ForegroundColor Cyan }
function Write-Ok([string]$msg)   { Write-Host "  OK $msg" -ForegroundColor Green }
function Write-Warn([string]$msg) { Write-Host "  !! $msg" -ForegroundColor Yellow }

function Test-Bundled([string[]]$names) {
    foreach ($name in $names) {
        if (-not (Test-Path -LiteralPath (Join-Path $OutputDir $name) -PathType Leaf)) { return $false }
    }
    return $true
}

function Get-GitHubAsset([string]$repo, [string]$assetRegex) {
    $api = "https://api.github.com/repos/$repo/releases/latest"
    $headers = @{ 'User-Agent' = 'livepaper-bundler/1.0' }
    # CI passes GITHUB_TOKEN to avoid the anonymous API rate limit; it is only
    # sent to api.github.com, never to the asset download host.
    if ($env:GITHUB_TOKEN) { $headers['Authorization'] = "Bearer $env:GITHUB_TOKEN" }
    $release = Invoke-RestMethod -Uri $api -Headers $headers
    $asset = $release.assets | Where-Object { $_.name -match $assetRegex } | Select-Object -First 1
    if (-not $asset) {
        throw "No asset matching '$assetRegex' found in $repo latest release"
    }
    return $asset
}

function Save-GitHubAsset($asset, [string]$outFile) {
    Write-Step "Downloading $($asset.name)"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $outFile -UseBasicParsing

    # GitHub publishes a sha256 digest for newer release assets; verify it when present.
    $digest = [string]$asset.digest
    if ($digest -match '^sha256:([0-9a-fA-F]{64})$') {
        $actual = (Get-FileHash -LiteralPath $outFile -Algorithm SHA256).Hash
        if ($actual -ne $Matches[1].ToUpperInvariant()) {
            throw "SHA-256 mismatch for $($asset.name): expected $($Matches[1]), got $actual"
        }
        Write-Ok "SHA-256 verified: $($asset.name)"
        return "$actual (verified against the GitHub release digest)"
    }

    if ($RequireDigest) {
        throw "No published digest for $($asset.name) and -RequireDigest was set"
    }
    $actual = (Get-FileHash -LiteralPath $outFile -Algorithm SHA256).Hash
    Write-Warn "No published digest for $($asset.name); recording computed SHA-256 $actual"
    return "$actual (computed at download; GitHub published no digest)"
}

function Expand-ZipTo([string]$zipPath, [string]$destDir) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $destDir)
}

function Get-7zr {
    # 7zr.exe (standalone 7-Zip reducer) handles .7z archives only — sufficient for mpv.
    $sevenZr = Join-Path $WorkDir '7zr.exe'
    if (Test-Path -LiteralPath $sevenZr) { return $sevenZr }

    Write-Step "Fetching 7zr.exe bootstrap from ip7z/7zip..."
    $asset = Get-GitHubAsset 'ip7z/7zip' '^7zr\.exe$'
    Save-GitHubAsset $asset $sevenZr | Out-Null
    return $sevenZr
}

function Expand-7zTo([string]$archivePath, [string]$destDir, [string]$sevenZr) {
    New-Item -ItemType Directory -Force -Path $destDir | Out-Null
    & $sevenZr x $archivePath "-o$destDir" -y | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "7z extraction failed (exit $LASTEXITCODE)" }
}

function Copy-Licenses([string]$searchDir, [string]$prefix) {
    $files = Get-ChildItem -LiteralPath $searchDir -Recurse -File |
        Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|COPYRIGHT)' }
    if (-not $files) {
        Write-Warn "No upstream license file in the $prefix archive; the source note links the upstream license"
        return
    }
    New-Item -ItemType Directory -Force -Path $LicenseDir | Out-Null
    foreach ($file in $files) {
        Copy-Item -LiteralPath $file.FullName (Join-Path $LicenseDir "$prefix-$($file.Name)") -Force
    }
    Write-Ok "Copied $(@($files).Count) $prefix license file(s) to $LicenseDir"
}

function Write-SourceNote([string]$prefix, [string]$repo, $asset, [string]$sha256, [string]$licenseUrl) {
    New-Item -ItemType Directory -Force -Path $LicenseDir | Out-Null
    $lines = @(
        "Bundled by scripts/bundle-media-tools.ps1"
        "Build repository: https://github.com/$repo"
        "Archive: $($asset.name)"
        "Download: $($asset.browser_download_url)"
        "SHA-256: $sha256"
        "Upstream license: $licenseUrl"
    )
    # WriteAllLines emits UTF-8 without a BOM on both PowerShell 5.1 and 7.
    [System.IO.File]::WriteAllLines((Join-Path $LicenseDir "$prefix-SOURCE.txt"), [string[]]$lines)
}

# ── Bundle ffmpeg + ffprobe ────────────────────────────────────────────────────

function Add-FFmpeg {
    if (!$Force -and (Test-Bundled $FFmpegFiles)) {
        Write-Ok "ffmpeg + ffprobe already bundled in $OutputDir"
        return
    }

    Write-Host "`n[ffmpeg]" -ForegroundColor White

    # BtbN/FFmpeg-Builds (zip, static GPL build)
    $repo = 'BtbN/FFmpeg-Builds'
    Write-Step "Fetching latest ffmpeg release from $repo..."
    $asset = Get-GitHubAsset $repo '^ffmpeg-N-\d+-g[0-9a-f]+-win64-gpl\.zip$'
    $zipPath = Join-Path $WorkDir 'ffmpeg.zip'
    $extractDir = Join-Path $WorkDir 'ffmpeg-extract'

    $sha256 = Save-GitHubAsset $asset $zipPath

    Write-Step "Extracting..."
    Expand-ZipTo $zipPath $extractDir

    $ffmpegExe = Get-ChildItem -LiteralPath $extractDir -Filter 'ffmpeg.exe' -Recurse | Select-Object -First 1
    if (-not $ffmpegExe) { throw "ffmpeg.exe not found in extracted archive" }
    $ffBin = $ffmpegExe.DirectoryName
    if (-not (Test-Path -LiteralPath (Join-Path $ffBin 'ffprobe.exe'))) { throw "ffprobe.exe not found in extracted archive" }

    Copy-Item -LiteralPath (Join-Path $ffBin 'ffmpeg.exe')  $OutputDir -Force
    Copy-Item -LiteralPath (Join-Path $ffBin 'ffprobe.exe') $OutputDir -Force
    Copy-Licenses $extractDir 'ffmpeg'
    Write-SourceNote 'ffmpeg' $repo $asset $sha256 'https://ffmpeg.org/legal.html'

    Remove-Item -LiteralPath $zipPath, $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Ok "ffmpeg + ffprobe bundled to: $OutputDir"
}

# ── Bundle mpv ─────────────────────────────────────────────────────────────────

function Add-Mpv {
    if (!$Force -and (Test-Bundled $MpvFiles)) {
        Write-Ok "mpv already bundled in $OutputDir"
        return
    }

    Write-Host "`n[mpv]" -ForegroundColor White

    # shinchiro/mpv-winbuild-cmake (7z)
    $repo = 'shinchiro/mpv-winbuild-cmake'
    Write-Step "Fetching latest mpv release from $repo..."
    $asset = Get-GitHubAsset $repo 'mpv-x86_64-\d{8}-git[^.]*\.7z$'
    $archivePath = Join-Path $WorkDir 'mpv.7z'
    $extractDir  = Join-Path $WorkDir 'mpv-extract'

    $sha256 = Save-GitHubAsset $asset $archivePath

    $sevenZr = Get-7zr
    Write-Step "Extracting (7z)..."
    Expand-7zTo $archivePath $extractDir $sevenZr

    # Only mpv.exe is shipped: a bundled mpv.com would win the PATHEXT lookup for "mpv".
    $mpvExe = Get-ChildItem -LiteralPath $extractDir -Filter 'mpv.exe' -Recurse | Select-Object -First 1
    if (-not $mpvExe) { throw "mpv.exe not found in extracted archive" }

    Copy-Item -LiteralPath $mpvExe.FullName $OutputDir -Force
    Copy-Licenses $extractDir 'mpv'
    # The shinchiro archive ships no license file; record the license terms so the GPL binary never ships without them.
    if (-not (Get-ChildItem -LiteralPath $LicenseDir -Filter 'mpv-*' -File -ErrorAction SilentlyContinue | Where-Object { $_.Name -ne 'mpv-SOURCE.txt' })) {
        New-Item -ItemType Directory -Force -Path $LicenseDir | Out-Null
        [System.IO.File]::WriteAllLines((Join-Path $LicenseDir 'mpv-LICENSE-NOTICE.txt'), [string[]]@(
            'mpv is licensed under the GNU General Public License, version 2 or later (GPLv2+),'
            'as built by shinchiro/mpv-winbuild-cmake. The archive bundled here contains no license file.'
            'License text: https://www.gnu.org/licenses/old-licenses/gpl-2.0.txt'
            'Upstream copyright and license notes: https://github.com/mpv-player/mpv/blob/master/Copyright'
            'Corresponding source: see mpv-SOURCE.txt for the exact build repository and archive.'
        ))
    }
    Write-SourceNote 'mpv' $repo $asset $sha256 'https://github.com/mpv-player/mpv/blob/master/Copyright'

    Remove-Item -LiteralPath $archivePath, $extractDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Ok "mpv bundled to: $OutputDir"
}

# ── Main ───────────────────────────────────────────────────────────────────────

Write-Host "livepaper media tools bundler" -ForegroundColor White
Write-Host "Output dir: $OutputDir"
Write-Host ""

try {
    New-Item -ItemType Directory -Force -Path $OutputDir, $WorkDir | Out-Null

    Add-FFmpeg
    Add-Mpv

    $missing = @($FFmpegFiles + $MpvFiles | Where-Object { -not (Test-Bundled @($_)) })
    if ($missing.Count -gt 0) {
        throw "Bundle incomplete; missing in ${OutputDir}: $($missing -join ', ')"
    }

    Write-Host ""
    Write-Host "Done. ffmpeg, ffprobe, and mpv are bundled in $OutputDir" -ForegroundColor Green
} catch {
    Write-Host ""
    Write-Host "ERROR: $_" -ForegroundColor Red
    exit 1
} finally {
    Remove-Item -LiteralPath $WorkDir -Recurse -Force -ErrorAction SilentlyContinue
}
