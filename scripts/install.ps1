# SPDX-License-Identifier: MPL-2.0
#
# Install script for invowk — a dynamically extensible command runner.
#
# Usage:
#   irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex
#
# Environment variables:
#   INVOWK_VERSION        - Specific version to install (e.g., v1.0.0). Default: latest stable.
#   INSTALL_DIR           - Installation directory. Default: $env:LOCALAPPDATA\Programs\invowk.
#   INVOWK_NO_MODIFY_PATH - Set to 1 to skip automatic PATH modification.
#   GITHUB_TOKEN          - Optional GitHub token for API rate limit relief.
#
# Requirements:
#   - PowerShell 5.1+ (ships with Windows 10/11)
#
# Supported platforms:
#   - Windows (amd64)

# Wrap in a function to prevent partial execution when piped via irm | iex.
# PowerShell's parser requires balanced braces, so a truncated download produces
# a parse error rather than executing partial code.
function Install-Invowk {
    [CmdletBinding()]
    param()

    # -------------------------------------------------------------------------
    # Constants
    # -------------------------------------------------------------------------

    $GitHubRepo = 'invowk/invowk'
    $ReleasesApiUrl = "https://api.github.com/repos/$GitHubRepo/releases/latest"
    $ReleasesTagUrl = "https://api.github.com/repos/$GitHubRepo/releases/tags"
    $DownloadBaseUrl = "https://github.com/$GitHubRepo/releases/download"
    $BinaryName = 'invowk'

    # -------------------------------------------------------------------------
    # TLS Configuration
    # -------------------------------------------------------------------------

    # PowerShell 5.1 defaults to TLS 1.0, but GitHub requires TLS 1.2+.
    # On PowerShell 7+ (Core), TLS 1.2+ is the default; this line is a harmless no-op.
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    # Suppress the progress bar — it slows Invoke-WebRequest 10-100x in PS 5.1.
    $ProgressPreference = 'SilentlyContinue'

    # -------------------------------------------------------------------------
    # Color Support
    # -------------------------------------------------------------------------

    # Detect ANSI color support. SupportsVirtualTerminal is reliable on modern PS hosts,
    # but Windows Terminal (detected via WT_SESSION) supports ANSI codes even when the
    # host object does not report the capability.
    $script:UseColors = $false
    if ($Host.UI.SupportsVirtualTerminal -or $env:WT_SESSION) {
        $script:UseColors = $true
    }

    $ESC = [char]27
    function Get-AnsiCode {
        param([string]$Code)
        if ($script:UseColors) { return "$ESC[$Code" }
        return ''
    }

    $Red    = Get-AnsiCode '0;31m'
    $Green  = Get-AnsiCode '0;32m'
    $Yellow = Get-AnsiCode '0;33m'
    $Cyan   = Get-AnsiCode '0;36m'
    $Bold   = Get-AnsiCode '1m'
    $Reset  = Get-AnsiCode '0m'

    # -------------------------------------------------------------------------
    # Logging
    # -------------------------------------------------------------------------

    function Write-Log  { param([string]$Message) Write-Host "${Cyan}${Message}${Reset}" }
    function Write-Ok   { param([string]$Message) Write-Host "${Green}${Message}${Reset}" }
    function Write-Warn { param([string]$Message) Write-Host "${Yellow}WARNING: ${Message}${Reset}" }
    function Write-Err  { param([string]$Message) Write-Host "${Red}ERROR: ${Message}${Reset}" }

    function Stop-WithError {
        param([string]$Message)
        Write-Err $Message
        throw $Message
    }

    # -------------------------------------------------------------------------
    # Prerequisites
    # -------------------------------------------------------------------------

    function Test-Prerequisites {
        # Check PowerShell version.
        if ($PSVersionTable.PSVersion.Major -lt 5 -or
            ($PSVersionTable.PSVersion.Major -eq 5 -and $PSVersionTable.PSVersion.Minor -lt 1)) {
            Stop-WithError "PowerShell 5.1 or later is required. Current version: $($PSVersionTable.PSVersion)"
        }

        # Check that we're on Windows.
        $onWindows = $true
        if ($PSVersionTable.PSEdition -eq 'Core') {
            $onWindows = $IsWindows
        }

        if (-not $onWindows) {
            Write-Log "This installer is for Windows. For Linux/macOS, use:"
            Write-Log ""
            Write-Log "  ${Bold}curl -fsSL https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.sh | sh${Reset}"
            Write-Log ""
            throw 'This installer is for Windows only.'
        }
    }

    # -------------------------------------------------------------------------
    # Architecture Detection
    # -------------------------------------------------------------------------

    function Get-Architecture {
        # Handle 32-bit PowerShell running on 64-bit Windows.
        $arch = if ($env:PROCESSOR_ARCHITEW6432) {
            $env:PROCESSOR_ARCHITEW6432
        } else {
            $env:PROCESSOR_ARCHITECTURE
        }

        switch ($arch) {
            'AMD64' { return 'amd64' }
            'x86' {
                Stop-WithError @"
32-bit Windows is not supported.
Invowk requires a 64-bit (x86_64/amd64) Windows installation.
"@
            }
            'ARM64' {
                Stop-WithError @"
Windows ARM64 is not currently supported.
Consider using: go install github.com/$GitHubRepo@latest
"@
            }
            default {
                Stop-WithError "Unsupported architecture: $arch"
            }
        }
    }

    # -------------------------------------------------------------------------
    # Web Requests
    # -------------------------------------------------------------------------

    function Invoke-SafeWebRequest {
        param(
            [string]$Uri,
            [string]$OutFile
        )

        $headers = @{
            'Accept' = 'application/vnd.github+json'
        }
        if ($env:GITHUB_TOKEN) {
            $headers['Authorization'] = "Bearer $env:GITHUB_TOKEN"
        }

        $params = @{
            Uri             = $Uri
            Headers         = $headers
            UseBasicParsing = $true
            ErrorAction     = 'Stop'
        }
        if ($OutFile) {
            $params['OutFile'] = $OutFile
        }

        Invoke-WebRequest @params
    }

    # -------------------------------------------------------------------------
    # Version Resolution
    # -------------------------------------------------------------------------

    function Resolve-Version {
        $version = $env:INVOWK_VERSION

        if ($version) {
            # User-specified version — validate format and verify it exists.
            if ($version -notmatch '^v') {
                $version = "v$version"
            }
            Write-Log "Using specified version: ${Bold}${version}${Reset}"

            # Verify the release exists by checking the tag endpoint.
            try {
                $null = Invoke-SafeWebRequest -Uri "$ReleasesTagUrl/$version"
            }
            catch {
                Stop-WithError @"
Version $version not found.
Check available versions at: https://github.com/$GitHubRepo/releases
"@
            }

            return $version
        }

        # Query the latest stable release via the GitHub API.
        Write-Log 'Fetching latest stable version...'

        try {
            $response = Invoke-SafeWebRequest -Uri $ReleasesApiUrl
        }
        catch {
            $statusCode = $null
            if ($_.Exception.Response) {
                $statusCode = [int]$_.Exception.Response.StatusCode
            }
            if ($statusCode -eq 403) {
                Stop-WithError @"
GitHub API rate limit exceeded.

Try again in a few minutes, or specify a version directly:
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.ps1 | iex

Or set a GitHub token for higher rate limits:
  `$env:GITHUB_TOKEN='ghp_...'; irm https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.ps1 | iex
"@
            }
            Stop-WithError @"
Failed to fetch latest release from GitHub.

Check your network connection and try again.
If behind a firewall, specify a version directly:
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.ps1 | iex
"@
        }

        $release = $response.Content | ConvertFrom-Json
        $version = $release.tag_name

        if (-not $version) {
            # Check for API error message (e.g., rate limiting returns HTTP 200 with error body).
            if ($release.message) {
                Stop-WithError @"
GitHub API error: $($release.message)

This often happens due to API rate limiting for unauthenticated requests.
Try again in a few minutes, or specify a version directly:
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.ps1 | iex
"@
            }
            Stop-WithError 'Could not determine latest version from GitHub API response.'
        }

        # Validate the extracted version looks like a semver tag.
        if ($version -notmatch '^v\d') {
            Stop-WithError @"
Unexpected version format from GitHub API: $version
This may indicate an API change. Report at: https://github.com/$GitHubRepo/issues
"@
        }

        Write-Log "Latest stable version: ${Bold}${version}${Reset}"
        return $version
    }

    # -------------------------------------------------------------------------
    # Installation
    # -------------------------------------------------------------------------

    # Construct the asset filename. GoReleaser strips the 'v' prefix in filenames.
    function Get-AssetFilename {
        param(
            [string]$Version,
            [string]$Arch
        )
        $versionNoV = $Version -replace '^v', ''
        return "${BinaryName}_${versionNoV}_windows_${Arch}.zip"
    }

    # Verify a file's SHA256 hash against an expected value.
    function Test-Checksum {
        param(
            [string]$FilePath,
            [string]$Expected
        )
        $actual = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash
        # Get-FileHash returns uppercase; checksums.txt uses lowercase. Compare case-insensitively.
        if ($actual -ne $Expected.ToUpper()) {
            Stop-WithError @"
Checksum verification failed for $(Split-Path -Leaf $FilePath)

Expected: $Expected
Got:      $($actual.ToLower())

The download may be corrupted. Please try again.
If this persists, report at https://github.com/$GitHubRepo/issues
"@
        }
    }

    function Install-Binary {
        param(
            [string]$Version,
            [string]$Arch,
            [string]$InstallDir
        )

        $asset = Get-AssetFilename -Version $Version -Arch $Arch
        $downloadUrl = "$DownloadBaseUrl/$Version/$asset"
        $checksumsUrl = "$DownloadBaseUrl/$Version/checksums.txt"

        # Create temp directory for staging.
        $tempDir = Join-Path ([IO.Path]::GetTempPath()) "invowk-install-$([IO.Path]::GetRandomFileName())"
        New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

        try {
            $archivePath = Join-Path $tempDir $asset
            $checksumsPath = Join-Path $tempDir 'checksums.txt'

            # Download the archive.
            Write-Log "Downloading ${Bold}${BinaryName} ${Version}${Reset} for windows/${Arch}..."
            try {
                Invoke-SafeWebRequest -Uri $downloadUrl -OutFile $archivePath
            }
            catch {
                Stop-WithError @"
Failed to download $asset.

The release asset may not exist for your platform (windows/$Arch).
Check available assets at: https://github.com/$GitHubRepo/releases/tag/$Version
"@
            }

            # Note: checksums.txt is not signature-verified by this script because cosign
            # is not a standard system tool. For supply-chain verification, see:
            # https://github.com/invowk/invowk#verifying-signatures
            Write-Log 'Downloading checksums...'
            try {
                Invoke-SafeWebRequest -Uri $checksumsUrl -OutFile $checksumsPath
            }
            catch {
                Stop-WithError @"
Failed to download checksums.txt.

Cannot verify download integrity. Please try again.
"@
            }

            # Extract expected checksum for our asset from checksums.txt.
            $checksumLine = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($asset) }
            if ($checksumLine -is [array]) {
                # Multiple matches found — narrow to exact asset name match.
                $checksumLine = $checksumLine | Where-Object { ($_ -split '\s+')[1] -eq $asset }
            }
            if (-not $checksumLine) {
                Stop-WithError @"
Asset $asset not found in checksums.txt.
This may indicate a GoReleaser configuration issue.
Report at: https://github.com/$GitHubRepo/issues
"@
            }
            $expectedHash = ($checksumLine -split '\s+')[0]

            # Verify the archive checksum.
            Write-Log 'Verifying checksum...'
            Test-Checksum -FilePath $archivePath -Expected $expectedHash
            Write-Ok 'Checksum verified.'

            # Extract the archive.
            Write-Log 'Extracting binary...'
            $extractDir = Join-Path $tempDir 'extracted'
            Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

            $binaryPath = Join-Path $extractDir "${BinaryName}.exe"
            if (-not (Test-Path $binaryPath)) {
                Stop-WithError "Binary '${BinaryName}.exe' not found in archive $asset."
            }

            # Create install directory if it doesn't exist.
            if (-not (Test-Path $InstallDir)) {
                try {
                    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
                }
                catch {
                    Stop-WithError @"
Failed to create install directory: $InstallDir
Try running with a different INSTALL_DIR or check permissions.
"@
                }
            }

            # Remove existing binary if present (Windows cannot overwrite a locked file).
            $destPath = Join-Path $InstallDir "${BinaryName}.exe"
            if (Test-Path $destPath) {
                try {
                    Remove-Item $destPath -Force
                }
                catch {
                    Stop-WithError @"
Failed to remove existing binary at $destPath.
The file may be in use. Close any running invowk processes and try again.
"@
                }
            }

            # Move binary to install directory.
            try {
                Move-Item -Path $binaryPath -Destination $destPath -Force
            }
            catch {
                Stop-WithError @"
Failed to install binary to $destPath

Try running with a different INSTALL_DIR:
  `$env:INSTALL_DIR='C:\tools\invowk'; irm https://raw.githubusercontent.com/$GitHubRepo/main/scripts/install.ps1 | iex
"@
            }

            Write-Ok "Successfully installed ${Bold}${BinaryName} ${Version}${Reset} to $destPath"
        }
        finally {
            # Cleanup temp directory.
            if (Test-Path $tempDir) {
                Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
            }
        }
    }

    # -------------------------------------------------------------------------
    # PATH Management
    # -------------------------------------------------------------------------

    function Update-Path {
        param([string]$InstallDir)

        if ($env:INVOWK_NO_MODIFY_PATH -eq '1') {
            return
        }

        # Check if install directory is already in User PATH.
        $currentPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if (-not $currentPath) { $currentPath = '' }
        $normalizedInstallDir = $InstallDir.TrimEnd('\', '/')
        $alreadyInPath = $currentPath -split ';' |
            Where-Object { $_ -ne '' } |
            Where-Object { $_.TrimEnd('\', '/') -eq $normalizedInstallDir }

        if ($alreadyInPath) {
            return
        }

        # Add to User PATH.
        try {
            $newPath = if ($currentPath) { "$currentPath;$InstallDir" } else { $InstallDir }
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            # Also update the current session's PATH so verification works immediately.
            $env:Path = "$env:Path;$InstallDir"
            Write-Ok "Added $InstallDir to your User PATH."
            Write-Log 'Restart your terminal for the PATH change to take effect in new sessions.'
        }
        catch {
            Write-Warn @"
Could not add $InstallDir to your PATH automatically.

Add it manually by running:
  [Environment]::SetEnvironmentVariable('Path', `$env:Path + ';$InstallDir', 'User')
"@
        }
    }

    # -------------------------------------------------------------------------
    # Main Flow
    # -------------------------------------------------------------------------

    Write-Log "Installing ${Bold}${BinaryName}${Reset}..."
    Write-Log ''

    Test-Prerequisites

    $arch = Get-Architecture

    # Resolve installation directory.
    $installDir = if ($env:INSTALL_DIR) {
        $env:INSTALL_DIR
    } else {
        Join-Path $env:LOCALAPPDATA 'Programs\invowk'
    }

    # Resolve target version.
    $version = Resolve-Version

    # Perform installation.
    Install-Binary -Version $version -Arch $arch -InstallDir $installDir

    # PATH management.
    Update-Path -InstallDir $installDir

    # Verify installation.
    $installedBinary = Join-Path $installDir "${BinaryName}.exe"
    if (Test-Path $installedBinary) {
        try {
            $installedVersion = & $installedBinary --version 2>&1
            Write-Ok ''
            Write-Ok "Verify: ${Bold}${BinaryName} --version${Reset} -> $installedVersion"
        }
        catch {
            Write-Log ''
            Write-Log "Run '${Bold}${BinaryName} --version${Reset}' to verify the installation."
        }
    }
}

# Invoke the installer.
Install-Invowk
