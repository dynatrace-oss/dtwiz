# install.ps1 — Download and install dtwiz on Windows.
#
# Usage:
#   .\install.ps1 [-InstallDir <dir>]
#
# By default the binary is installed to $env:LOCALAPPDATA\Programs\dtwiz.
# Pass -InstallDir to override.  The install directory is added permanently
# to the current user's PATH.
#
# The script requires an internet connection (uses Invoke-WebRequest).
#
# Run once to allow execution:
#   Set-ExecutionPolicy -Scope CurrentUser -ExecutionPolicy RemoteSigned

[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [string]$Branch = $env:DTWIZ_BRANCH
)

$ErrorActionPreference = "Stop"
$Repo = "dynatrace-oss/dtwiz"

# ── Detect architecture ────────────────────────────────────────────────────────
# Try .NET RuntimeInformation first (PowerShell 7+ / .NET 4.7.1+), then fall
# back to the PROCESSOR_ARCHITECTURE environment variable (always set on Windows).
$RawArch = $null
try {
    $RawArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
} catch {
    # Property not available — fall back below.
}
if (-not $RawArch) {
    $RawArch = $env:PROCESSOR_ARCHITECTURE   # AMD64 | ARM64 | x86
}

switch ($RawArch) {
    { $_ -in "X64", "AMD64" }  { $Arch = "amd64" }
    { $_ -in "Arm64", "ARM64" } { $Arch = "arm64" }
    default {
        Write-Error "Unsupported architecture: $RawArch"
        exit 1
    }
}

Write-Host "Detected platform: windows/$Arch"

# ── Resolve release version ────────────────────────────────────────────────────
if ($Branch) {
    # Derive the pre-release tag from the branch name (e.g. preview/foo -> snapshot-preview-foo)
    $ReleaseTag = "snapshot-" + ($Branch -replace '/', '-')
    Write-Host "Installing preview snapshot for branch: $Branch"
    $Version = (Invoke-WebRequest `
        -Uri "https://github.com/$Repo/releases/download/$ReleaseTag/version.txt" `
        -UseBasicParsing).Content.Trim()
    if (-not $Version) {
        Write-Error "Could not find a snapshot release for branch '$Branch'. Make sure the branch exists and its snapshot workflow has completed."
        exit 1
    }
} else {
    # Follow the /releases/latest redirect to extract the tag from the final URL.
    $Response = Invoke-WebRequest `
        -Uri "https://github.com/$Repo/releases/latest" `
        -MaximumRedirection 0 `
        -ErrorAction SilentlyContinue `
        -UseBasicParsing
    $RedirectUrl = $Response.Headers.Location
    if (-not $RedirectUrl) {
        # Some PS versions follow the redirect automatically
        $RedirectUrl = $Response.BaseResponse.ResponseUri.AbsoluteUri
        if (-not $RedirectUrl) {
            $RedirectUrl = $Response.BaseResponse.RequestMessage.RequestUri.AbsoluteUri
        }
    }
    $ReleaseTag = ($RedirectUrl -split '/')[-1]
    $Version = $ReleaseTag
    if (-not $Version) {
        Write-Error "Could not determine the latest dtwiz version."
        exit 1
    }
}

# ── Determine install directory ────────────────────────────────────────────────
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\dtwiz"
}

# ── Confirm installation ──────────────────────────────────────────────────────
Write-Host ""
Write-Host "This will download and install dtwiz ${Version}:"
if ($Branch) {
    Write-Host "  * Branch:   $Branch (pre-release)"
}
Write-Host "  * Download from github.com/${Repo}"
Write-Host "  * Install to $InstallDir"
Write-Host "  * Add $InstallDir to your user PATH (if not already present)"
Write-Host ""
$Confirm = Read-Host "Continue? [Y/n]"
if ($Confirm -match '^[Nn]') {
    Write-Host "Installation cancelled."
    exit 0
}

# ── Download and extract ───────────────────────────────────────────────────────
Write-Host ""
Write-Host "Downloading dtwiz ${Version}..."

$VersionNum = $Version.TrimStart("v")
$Archive    = "dtwiz_${VersionNum}_windows_${Arch}.zip"
$TmpDir     = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    $ArchivePath = Join-Path $TmpDir $Archive

    $DownloadUrl = "https://github.com/$Repo/releases/download/$ReleaseTag/$Archive"
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing

    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

    $ExtractedBinary = Join-Path $TmpDir "dtwiz.exe"
    if (-not (Test-Path $ExtractedBinary)) {
        Write-Error "dtwiz.exe not found after extraction."
        exit 1
    }

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # ── Install binary ─────────────────────────────────────────────────────────
    $Dest = Join-Path $InstallDir "dtwiz.exe"
    Move-Item -Force $ExtractedBinary $Dest

    Write-Host ""
    Write-Host "dtwiz ${Version} installed to ${Dest}"

    # ── Add to user PATH if needed ─────────────────────────────────────────────
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    $PathDirs = $UserPath -split ";"
    if ($PathDirs -notcontains $InstallDir) {
        $NewPath = ($PathDirs + $InstallDir) -join ";"
        [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
        # Also update the current session
        $env:PATH = "$env:PATH;$InstallDir"
        Write-Host ""
        Write-Host "  Added $InstallDir to your user PATH."
        Write-Host "  Open a new terminal for the change to take effect."
    }
} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
