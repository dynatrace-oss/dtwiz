# install.ps1 - Download and install dtwiz on Windows.
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
#
# Install channels:
#   (default)              Latest stable release
#   -Channel main          Latest snapshot from the main branch
#   -Tag <tag>             Specific snapshot/pre-release tag
#
# Self-monitoring:
#   When DT_ENVIRONMENT is set, the script sends a self-monitoring event to that
#   Dynatrace environment when it starts and when it completes, fails, or is
#   cancelled. DT_PLATFORM_TOKEN is used for authorization when set.

[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [string]$Tag = $env:DTWIZ_TAG,
    [string]$Channel = $env:DTWIZ_CHANNEL
)

# Make failing cmdlets (Invoke-WebRequest, Expand-Archive, Move-Item, ...) throw,
# so that every failure reaches the catch block at the end of the script.
$ErrorActionPreference = "Stop"
$Repo = "dynatrace-oss/dtwiz"

# Translate channel name to a release tag
if ($Channel -eq "main") {
    $Tag = "snapshot-main"
}

# -- Self-monitoring ------------------------------------------------------------
# Execution id correlating the events of one run: 3 random hex chars.
$SmExecId = '{0:x3}' -f (Get-Random -Maximum 4096)

# $IsLinux and $IsMacOS exist from PowerShell 6 on. Windows PowerShell 5.1 does
# not define them, but only runs on Windows.
if ($IsLinux) {
    $SmOs = "linux"; $SmOsShort = "lin"
} elseif ($IsMacOS) {
    $SmOs = "darwin"; $SmOsShort = "mac"
} else {
    $SmOs = "windows"; $SmOsShort = "win"
}

if ($Channel -eq "main") {
    $SmChannel = "main"
} elseif ($Tag) {
    $SmChannel = "tag"
} else {
    $SmChannel = "stable"
}

# Resolved dtwiz version without the leading "v"; empty until resolved.
$SmVersion = ""
# Failure reason reported if the current stage throws.
$FailReason = ""
# Set once an outcome (completed, failed, cancelled) is reported, so exactly one
# outcome event is sent per run.
$SmOutcomeSent = $false

# Sends a self-monitoring event to DT_ENVIRONMENT. Silent, bounded to 3 seconds,
# and a no-op when DT_ENVIRONMENT is not set. Never affects the install.
function Send-InstallEvent {
    param(
        [string]$Step,
        [string]$Reason = ""
    )
    if (-not $env:DT_ENVIRONMENT) { return }
    try {
        $StepNames = @{ inv = "invoked"; com = "completed"; fai = "failed"; can = "cancelled" }

        # Same normalization as the binary: classic URL, no trailing slash.
        $Url = ($env:DT_ENVIRONMENT -replace '\.apps\.', '.').TrimEnd('/') + "/api/v2/events/ingest"

        $UserAgent = "dtwiz/install;s=ps1;st=$Step"
        if ($SmVersion) { $UserAgent += ";v=$SmVersion" }
        if ($Reason) { $UserAgent += ";er=$Reason" }

        $Props = [ordered]@{
            executionId = $SmExecId
            script      = "ps1"
            step        = $StepNames[$Step]
            os          = $SmOs
            channel     = $SmChannel
        }
        if ($Tag) { $Props.tag = $Tag }
        if ($SmVersion) { $Props.version = $SmVersion }
        if ($Reason) { $Props.error = $Reason }
        $Body = @{
            eventType  = "CUSTOM_INFO"
            title      = "dtwiz install script"
            properties = $Props
        } | ConvertTo-Json -Compress

        # Without a token the event is still sent: the environment's edge records
        # the headers even when ingest rejects it.
        $Headers = @{
            "Tab-Id"           = "$SmExecId;o=$SmOsShort"
            "dtwiz-monitoring" = "dtwiz-start"
        }
        if ($env:DT_PLATFORM_TOKEN) {
            $Headers["Authorization"] = "Bearer $($env:DT_PLATFORM_TOKEN)"
        }

        Invoke-WebRequest `
            -Uri $Url `
            -Method Post `
            -Body $Body `
            -ContentType "application/json" `
            -UserAgent $UserAgent `
            -Headers $Headers `
            -TimeoutSec 3 `
            -UseBasicParsing `
            -ErrorAction Stop | Out-Null
    } catch {
        # Self-monitoring must never surface to the user.
    }
}

# Fails with github_unreachable when a GitHub request got no answer at all (DNS,
# connect, timeout, TLS, proxy). An HTTP error status means GitHub answered;
# the caller then reports release_not_found. Both Windows PowerShell 5.1 and
# PowerShell 7 attach the response to the exception only when one was received.
function Assert-GitHubAnswered {
    param($ErrorRecord)
    if (-not $ErrorRecord.Exception.Response) {
        $script:FailReason = "github_unreachable"
        throw "Could not reach github.com: $($ErrorRecord.Exception.Message) Check your network connection and proxy settings."
    }
}

Send-InstallEvent "inv"

try {
    # -- Detect OS --------------------------------------------------------------
    $FailReason = "unsupported_os"
    if ($SmOs -ne "windows") {
        throw "Unsupported OS: $SmOs. Use install.sh on Linux and macOS."
    }

    # -- Detect architecture ----------------------------------------------------
    # Try .NET RuntimeInformation first (PowerShell 7+ / .NET 4.7.1+), then fall
    # back to the PROCESSOR_ARCHITECTURE environment variable (always set on Windows).
    $FailReason = "unsupported_arch"
    $RawArch = $null
    try {
        $RawArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        # Property not available - fall back below.
    }
    if (-not $RawArch) {
        $RawArch = $env:PROCESSOR_ARCHITECTURE   # AMD64 | ARM64 | x86
    }

    switch ($RawArch) {
        { $_ -in "X64", "AMD64" }  { $Arch = "amd64" }
        { $_ -in "Arm64", "ARM64" } { $Arch = "arm64" }
        default {
            throw "Unsupported architecture: $RawArch"
        }
    }

    Write-Host "Detected platform: windows/$Arch"

    # -- Resolve release version ------------------------------------------------
    $FailReason = "release_not_found"
    if ($Tag) {
        # Use the provided release tag directly
        $ReleaseTag = $Tag
        if ($Tag -eq "snapshot-main") {
            Write-Host "Installing latest snapshot from the main branch..."
        } else {
            Write-Host "Installing preview snapshot for tag: $Tag"
        }
        $VersionFile = Join-Path ([System.IO.Path]::GetTempPath()) "dtwiz-version.txt"
        $Version = ""
        try {
            Invoke-WebRequest `
                -Uri "https://github.com/$Repo/releases/download/$ReleaseTag/version.txt" `
                -OutFile $VersionFile `
                -UseBasicParsing
            $Version = (Get-Content $VersionFile -Raw).Trim()
            Remove-Item $VersionFile -ErrorAction SilentlyContinue
        } catch {
            Assert-GitHubAnswered $_
            # GitHub answered with an error status: handled by the empty-version check below.
        }
        if (-not $Version) {
            if ($Tag -eq "snapshot-main") {
                throw "Could not find the main snapshot release. The 'snapshot-main' release may not have been published yet."
            }
            throw "Could not find a snapshot release for tag '$Tag'. Make sure the tag exists and its release has been published."
        }
    } else {
        # Resolve the tag behind /releases/latest by letting Invoke-WebRequest follow
        # the redirect normally, then reading the final resolved URL back off the
        # response. We deliberately don't disable redirects and inspect the Location
        # header instead: the shape of a non-success response (and whether
        # -ErrorAction can suppress the resulting error) differs between Windows
        # PowerShell 5.1 and PowerShell 7+, so relying on it is fragile.
        try {
            $Response = Invoke-WebRequest `
                -Uri "https://github.com/$Repo/releases/latest" `
                -UseBasicParsing
        } catch {
            Assert-GitHubAnswered $_
            throw "Could not determine the latest dtwiz release: $($_.Exception.Message)"
        }
        $RedirectUrl = $Response.BaseResponse.ResponseUri.AbsoluteUri
        if (-not $RedirectUrl) {
            $RedirectUrl = $Response.BaseResponse.RequestMessage.RequestUri.AbsoluteUri
        }
        $ReleaseTag = ($RedirectUrl -split '/')[-1]
        $Version = $ReleaseTag
        # A failed lookup can leave the original ".../releases/latest" URL behind,
        # so require something that looks like a version tag.
        if ($Version -notmatch '^v[0-9]') {
            throw "Could not determine the latest dtwiz release."
        }
    }

    $VersionNum = $Version.TrimStart("v")
    $SmVersion  = $VersionNum

    # -- Determine install directory --------------------------------------------
    $FailReason = "install_dir_failed"
    if (-not $InstallDir) {
        if (-not $env:LOCALAPPDATA) {
            throw "LOCALAPPDATA is not set. Pass -InstallDir to choose the install directory."
        }
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\dtwiz"
    }

    # -- Confirm installation ---------------------------------------------------
    Write-Host ""
    Write-Host "This will download and install dtwiz ${Version}:"
    if ($Tag -eq "snapshot-main") {
        Write-Host "  * Channel:  main (pre-release snapshot)"
    } elseif ($Tag) {
        Write-Host "  * Tag:      $Tag (pre-release)"
    }
    Write-Host "  * Download from github.com/${Repo}"
    Write-Host "  * Install to $InstallDir"
    Write-Host "  * Add $InstallDir to your user PATH (if not already present)"
    Write-Host ""
    # Read-Host throws in sessions that cannot prompt (-NonInteractive, CI).
    $FailReason = "prompt_failed"
    $Confirm = Read-Host "Continue? [Y/n]"
    if ($Confirm -match '^[Nn]') {
        Write-Host "Installation cancelled."
        Send-InstallEvent "can"
        $SmOutcomeSent = $true
        exit 0
    }

    # -- Download and extract ---------------------------------------------------
    Write-Host ""
    Write-Host "Downloading dtwiz ${Version}..."

    $FailReason = "download_failed"
    $Archive    = "dtwiz_${VersionNum}_windows_${Arch}.zip"
    $TmpDir     = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $TmpDir | Out-Null

    try {
        $ArchivePath = Join-Path $TmpDir $Archive

        $DownloadUrl = "https://github.com/$Repo/releases/download/$ReleaseTag/$Archive"
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ArchivePath -UseBasicParsing

        $FailReason = "extract_failed"
        Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

        $FailReason = "binary_missing"
        $ExtractedBinary = Join-Path $TmpDir "dtwiz.exe"
        if (-not (Test-Path $ExtractedBinary)) {
            throw "dtwiz.exe not found after extraction."
        }

        $FailReason = "move_failed"
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir | Out-Null
        }

        # -- Install binary -----------------------------------------------------
        $Dest = Join-Path $InstallDir "dtwiz.exe"
        $OldDest = "$Dest.old"
        # Leftover from a previous upgrade whose old binary was still running.
        Remove-Item -Force $OldDest -ErrorAction SilentlyContinue
        if (Test-Path $Dest) {
            # A running dtwiz.exe cannot be overwritten or deleted, but it can be
            # renamed. Move it aside so the new binary can take its place.
            Move-Item -Force $Dest $OldDest
        }
        Move-Item -Force $ExtractedBinary $Dest
        # Still locked if dtwiz is running; removed on the next install instead.
        Remove-Item -Force $OldDest -ErrorAction SilentlyContinue

        Write-Host ""
        Write-Host "dtwiz ${Version} installed to ${Dest}"
        Send-InstallEvent "com"
        $SmOutcomeSent = $true

        # -- Add to user PATH if needed -----------------------------------------
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
        Write-Host ""
        Write-Host "┌────────────────────────────────────────────────────────────────┐"
        Write-Host "│ ℹ️  In case dtwiz is blocked by Windows Security               │"
        Write-Host "├────────────────────────────────────────────────────────────────┤"
        Write-Host "│ 1. Run:                                                        │"
        Write-Host "│    Start-Process `"windowsdefender://exclusions`"                │"
        Write-Host "│                                                                │"
        Write-Host "│ 2. Add this folder to exclusions:                              │"
        $DefenderPad = " " * ([Math]::Max(0, 60 - $InstallDir.Length))
        Write-Host "│    $InstallDir$DefenderPad│"
        Write-Host "│                                                                │"
        Write-Host "│ Then re-run `"dtwiz setup`".                                     │"
        Write-Host "└────────────────────────────────────────────────────────────────┘"
    } finally {
        Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
    }
} catch {
    # Every failure above lands here: report it with the reason of the stage
    # that was running, then rethrow so the user sees the error.
    if (-not $SmOutcomeSent) {
        Send-InstallEvent "fai" $FailReason
        $SmOutcomeSent = $true
    }
    throw
} finally {
    # Ctrl+C stops the script without running catch blocks, but finally blocks
    # still run. A run that gets here without an outcome was interrupted.
    if (-not $SmOutcomeSent) {
        Send-InstallEvent "can" "interrupted"
    }
}
