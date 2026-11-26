# SPDX-License-Identifier: MPL-2.0
#
# Install script for invowk — a dynamically extensible command runner.
#
# Usage:
#   irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex
#
# Environment variables:
#   INVOWK_VERSION           - Specific version to install (e.g., v1.0.0). Default: latest stable.
#   INSTALL_DIR              - Installation directory. Default: $env:LOCALAPPDATA\Programs\invowk.
#   INVOWK_NO_MODIFY_PATH    - Set to 1 to skip automatic PATH modification.
#   INVOWK_REQUIRE_SIGNATURE - Set to 1, true or yes to fail when cosign is not
#                              installed, instead of warning and installing without
#                              verifying the signature of checksums.txt. 0, false, no
#                              or unset keep the warning; any other value is an error.
#   GITHUB_TOKEN             - Optional GitHub token for API rate limit relief.
#
# Requirements:
#   - PowerShell 5.1+ (ships with Windows 10/11)
#   - cosign (optional, recommended): verifies the Sigstore signature of
#     checksums.txt before the script trusts it. Releases with a Sigstore
#     bundle (v0.5.0 on) need cosign v2.4.2 or later.
#
# Supported platforms:
#   - Windows (amd64)

# -------------------------------------------------------------------------
# Constants (script scope — shared between Install-Invowk and testable functions)
# -------------------------------------------------------------------------

$script:GitHubRepo = 'invowk/invowk'
$script:BinaryName = 'invowk'

# The only signer accepted for checksums.txt: the release workflow, run by a
# tag push, at the tag of the version being installed. Other workflows of the
# repository can also obtain an OIDC token, so the identity is pinned to the
# workflow file and the tag, never to the repository alone.
$script:SignerIdentityPrefix = "https://github.com/$($script:GitHubRepo)/.github/workflows/release.yml@refs/tags/"
$script:SignerOidcIssuer = 'https://token.actions.githubusercontent.com'
$script:SignerWorkflowTrigger = 'push'

# The oldest cosign that verifies every published checksums.txt.sigstore.json
# (v0.7.0+ publish Sigstore bundle v0.3 format, rejected by cosign <= v2.4.1).
$script:CosignMinBundleVersion = '2.4.2'

# -------------------------------------------------------------------------
# Testable Pure Functions (script scope — accessible when dot-sourced)
# -------------------------------------------------------------------------

# Construct the asset filename. GoReleaser strips the 'v' prefix in filenames.
function Get-AssetFilename {
    param(
        [string]$Version,
        [string]$Arch
    )
    $versionNoV = $Version -replace '^v', ''
    return "${script:BinaryName}_${versionNoV}_windows_${Arch}.zip"
}

# Detect the host CPU architecture and map to Go's naming convention.
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
            throw @"
32-bit Windows is not supported.
Invowk requires a 64-bit (x86_64/amd64) Windows installation.
"@
        }
        'ARM64' {
            throw @"
Windows ARM64 is not currently supported.
Consider using: go install github.com/${script:GitHubRepo}@latest
"@
        }
        default {
            throw "Unsupported architecture: $arch"
        }
    }
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
        throw @"
Checksum verification failed for $(Split-Path -Leaf $FilePath)

Expected: $Expected
Got:      $($actual.ToLower())

The download may be corrupted. Please try again.
If this persists, report at https://github.com/${script:GitHubRepo}/issues
"@
    }
}

# Check whether a directory is present in a semicolon-delimited path string.
# Normalizes trailing slashes for reliable comparison.
function Test-InPath {
    param(
        [string]$Dir,
        [string]$PathString
    )
    if (-not $PathString) { return $false }
    $normalizedDir = $Dir.TrimEnd('\', '/')
    $entries = $PathString -split ';' | Where-Object { $_ -ne '' }
    foreach ($entry in $entries) {
        if ($entry.TrimEnd('\', '/') -eq $normalizedDir) {
            return $true
        }
    }
    return $false
}

# Validate that a version string looks like a semver tag (vN.N.N with optional suffix)
# and contains only safe tag characters before it enters download URLs or signer pins.
function Test-VersionFormat {
    param([string]$Version)
    if ([string]::IsNullOrEmpty($Version)) { return $false }
    return [bool]($Version -cmatch '\Av[0-9]+\.[0-9]+\.[0-9]+[0-9A-Za-z.+-]*\z')
}

# Print the certificate identity that must have signed a version's checksums.txt.
function Get-SignerIdentity {
    param([string]$Version)
    return "$($script:SignerIdentityPrefix)$Version"
}

# Report whether INVOWK_REQUIRE_SIGNATURE asks for a verified signature.
# 1, true and yes (any case) mean on; 0, false, no, empty and unset mean off.
# Any other value is fatal, so a typo never turns the requirement off.
function Test-SignatureRequired {
    $value = $env:INVOWK_REQUIRE_SIGNATURE
    if ([string]::IsNullOrEmpty($value)) {
        return $false
    }
    switch -Exact ($value) {
        '1'     { return $true }
        'true'  { return $true }
        'yes'   { return $true }
        '0'     { return $false }
        'false' { return $false }
        'no'    { return $false }
        default {
            throw @"
Invalid INVOWK_REQUIRE_SIGNATURE value: $value
Use 1, true or yes to require a verified signature, or 0, false or no (or leave it unset) to allow installing without cosign.
"@
        }
    }
}

# Report whether version X.Y.Z ($Have) is at least version X.Y.Z ($Min).
function Test-VersionAtLeast {
    param(
        [string]$Have,
        [string]$Min
    )
    if ($Have -cnotmatch '\A[0-9]+\.[0-9]+\.[0-9]+\z' -or $Min -cnotmatch '\A[0-9]+\.[0-9]+\.[0-9]+\z') {
        return $false
    }
    $haveParts = $Have -split '\.'
    $minParts = $Min -split '\.'
    for ($i = 0; $i -lt 3; $i++) {
        $h = [int]$haveParts[$i]
        $m = [int]$minParts[$i]
        if ($h -gt $m) { return $true }
        if ($h -lt $m) { return $false }
    }
    return $true
}

# Extract the X.Y.Z version from `cosign version` output, or '' when missing / devel.
function Get-CosignVersion {
    param([string]$Output)
    if ([string]::IsNullOrEmpty($Output)) { return '' }
    foreach ($line in ($Output -split "\r?\n")) {
        if ($line -cmatch '^GitVersion:\s*v?([0-9]+\.[0-9]+\.[0-9]+)') {
            return $Matches[1]
        }
    }
    return ''
}

# Report whether cosign output shows that cosign could not reach Sigstore services
# rather than a signature that does not verify. Identity/signature errors ALWAYS
# count as a bad signature so attacker-controlled SAN text cannot masquerade as
# a network failure.
function Test-CosignOutputIsNetworkError {
    param([string]$Output)
    if ([string]::IsNullOrEmpty($Output)) { return $false }
    if ($Output -match '(?i)CertificateIdentity|expected SAN|invalid signature|failed to verify|verification failed|no matching') {
        return $false
    }
    return [bool]($Output -match '(?i)tuf refresh failed|getting trusted root from TUF|failed to create TUF client|dial tcp|proxyconnect tcp|no such host|connection refused|connection reset by peer|i/o timeout|TLS handshake timeout|context deadline exceeded|network is unreachable')
}

# Run a native cosign command and capture exit code + combined stdout/stderr output
# safely under both Windows PowerShell 5.1 and PowerShell 7+ with ErrorActionPreference=Stop.
function Invoke-CosignCommand {
    param(
        [string]$CosignPath,
        [string[]]$Arguments
    )
    $prevErrorAction = $ErrorActionPreference
    $hasNativePref = Test-Path variable:PSNativeCommandUseErrorActionPreference
    $prevNativePref = $null
    if ($hasNativePref) {
        $prevNativePref = $PSNativeCommandUseErrorActionPreference
    }
    try {
        $ErrorActionPreference = 'Continue'
        if ($hasNativePref) {
            $PSNativeCommandUseErrorActionPreference = $false
        }
        $rawOutput = & $CosignPath @Arguments 2>&1
        $exitCode = if ($null -ne $LASTEXITCODE) { [int]$LASTEXITCODE } else { 0 }
        $lines = @()
        foreach ($item in $rawOutput) {
            if ($null -ne $item) {
                $lines += $item.ToString()
            }
        }
        return @{
            ExitCode = $exitCode
            Output   = ($lines -join "`n")
        }
    }
    finally {
        $ErrorActionPreference = $prevErrorAction
        if ($hasNativePref) {
            $PSNativeCommandUseErrorActionPreference = $prevNativePref
        }
    }
}

# Fail unless the cosign on PATH can verify a Sigstore bundle.
function Assert-CosignBundleSupport {
    param(
        [string]$CosignPath,
        [scriptblock]$RunCosign = { param($Exe, $CmdArgs) Invoke-CosignCommand -CosignPath $Exe -Arguments $CmdArgs },
        [scriptblock]$WriteWarning = { param($Msg) Write-Warning $Msg }
    )
    $versionResult = & $RunCosign $CosignPath @('version')
    if ($versionResult.ExitCode -ne 0) {
        throw @"
cosign is on PATH, but 'cosign version' failed, so the signature of checksums.txt cannot be verified.
This is a problem with the cosign installation, not with the release.
cosign output:
$($versionResult.Output)

Repair or upgrade cosign (v$($script:CosignMinBundleVersion) or later) and try again.
"@
    }
    $cosignVersion = Get-CosignVersion -Output $versionResult.Output
    if ([string]::IsNullOrEmpty($cosignVersion)) {
        & $WriteWarning "Could not read the cosign version; this release needs cosign v$($script:CosignMinBundleVersion) or later. Verifying anyway."
        return
    }
    if (-not (Test-VersionAtLeast -Have $cosignVersion -Min $script:CosignMinBundleVersion)) {
        throw "cosign $cosignVersion is older than the minimum $($script:CosignMinBundleVersion) needed to verify this release; upgrade cosign to v$($script:CosignMinBundleVersion) or later and try again."
    }
}

# Verify the Cosign signature of checksums.txt before trusting it.
function Test-ChecksumsSignature {
    param(
        [string]$Version,
        [string]$ChecksumsPath,
        [string]$DownloadBaseUrl = "https://github.com/$($script:GitHubRepo)/releases/download",
        [scriptblock]$FindCosign = {
            $cmd = Get-Command cosign -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($cmd) { return $cmd.Source }
            return $null
        },
        [scriptblock]$DownloadFile,
        [scriptblock]$RunCosign = { param($Exe, $CmdArgs) Invoke-CosignCommand -CosignPath $Exe -Arguments $CmdArgs },
        [scriptblock]$WriteInfo = { param($Msg) },
        [scriptblock]$WriteSuccess = { param($Msg) },
        [scriptblock]$WriteWarning = { param($Msg) Write-Warning $Msg }
    )

    $sigRequired = Test-SignatureRequired
    $cosignPath = & $FindCosign
    if ([string]::IsNullOrEmpty($cosignPath)) {
        if ($sigRequired) {
            throw @"
cosign not found, and INVOWK_REQUIRE_SIGNATURE=$($env:INVOWK_REQUIRE_SIGNATURE) requires the signature of checksums.txt to be verified.
Install cosign v$($script:CosignMinBundleVersion) or later (https://docs.sigstore.dev/cosign/system_config/installation/) and try again.
"@
        }
        & $WriteWarning @"
cosign not found: the signature of checksums.txt was NOT verified; only the SHA256 checksum of the archive will be checked.
Install cosign v$($script:CosignMinBundleVersion) or later (https://docs.sigstore.dev/cosign/system_config/installation/) to verify it,
or set INVOWK_REQUIRE_SIGNATURE=1 to make a missing cosign an error.
"@
        return
    }

    if (-not $DownloadFile) {
        throw 'Internal error: DownloadFile callback is required when cosign is present.'
    }

    $identity = Get-SignerIdentity -Version $Version
    $sigBaseUrl = "$DownloadBaseUrl/$Version/checksums.txt"
    $bundlePath = "$ChecksumsPath.sigstore.json"
    $sigPath = "$ChecksumsPath.sig"
    $pemPath = "$ChecksumsPath.pem"

    & $WriteInfo 'Verifying the signature of checksums.txt...'

    # Releases from v0.5.0 on publish a Sigstore bundle; earlier ones published
    # a detached signature and certificate. Once a bundle downloads, never fall
    # back to a detached signature.
    $haveBundle = $false
    try {
        & $DownloadFile "$sigBaseUrl.sigstore.json" $bundlePath
        if (Test-Path -LiteralPath $bundlePath) {
            $haveBundle = $true
        }
    }
    catch {
        Remove-Item -LiteralPath $bundlePath -Force -ErrorAction SilentlyContinue
        $haveBundle = $false
    }

    $sigArgs = $null
    if ($haveBundle) {
        Assert-CosignBundleSupport -CosignPath $cosignPath -RunCosign $RunCosign -WriteWarning $WriteWarning
        $sigArgs = @('--bundle', $bundlePath)
    }
    else {
        $haveLegacy = $false
        try {
            & $DownloadFile "$sigBaseUrl.sig" $sigPath
            & $DownloadFile "$sigBaseUrl.pem" $pemPath
            if ((Test-Path -LiteralPath $sigPath) -and (Test-Path -LiteralPath $pemPath)) {
                $haveLegacy = $true
            }
        }
        catch {
            Remove-Item -LiteralPath $sigPath, $pemPath -Force -ErrorAction SilentlyContinue
            $haveLegacy = $false
        }
        if ($haveLegacy) {
            $sigArgs = @('--signature', $sigPath, '--certificate', $pemPath)
        }
        else {
            throw @"
Could not download the signature of checksums.txt for $Version.

cosign is installed, so the installer will not continue without verifying it.
Check your network connection and try again.
If this persists, report at https://github.com/$($script:GitHubRepo)/issues
"@
        }
    }

    $verifyArgs = @(
        'verify-blob'
    ) + $sigArgs + @(
        '--certificate-identity', $identity,
        '--certificate-oidc-issuer', $script:SignerOidcIssuer,
        '--certificate-github-workflow-trigger', $script:SignerWorkflowTrigger,
        $ChecksumsPath
    )

    $verifyResult = & $RunCosign $cosignPath $verifyArgs
    if ($verifyResult.ExitCode -ne 0) {
        if (Test-CosignOutputIsNetworkError -Output $verifyResult.Output) {
            throw @"
cosign could not verify the signature of checksums.txt for ${Version}: it could not reach the Sigstore services (TUF metadata or Rekor).
This usually points to a network or proxy problem. Do not install this release
until its signature verifies; the installer does not continue without it.
cosign output:
$($verifyResult.Output)

Check your network connection and proxy settings, and try again.
"@
        }
        throw @"
Signature verification of checksums.txt failed for $Version.

Expected signer: $identity
Expected issuer: $($script:SignerOidcIssuer)
cosign output:
$($verifyResult.Output)

Do not install this release. Report at https://github.com/$($script:GitHubRepo)/issues
"@
    }

    & $WriteSuccess 'Signature of checksums.txt verified.'
}

# -------------------------------------------------------------------------
# Installer Orchestration
# -------------------------------------------------------------------------

# Wrap in a function to prevent partial execution when piped via irm | iex.
# PowerShell's parser requires balanced braces, so a truncated download produces
# a parse error rather than executing partial code.
function Install-Invowk {
    [CmdletBinding()]
    param()

    # Catch non-terminating errors from cmdlets like Expand-Archive, New-Item, Move-Item.
    $ErrorActionPreference = 'Stop'

    # -------------------------------------------------------------------------
    # Constants (installer-only)
    # -------------------------------------------------------------------------

    $ReleasesApiUrl = "https://api.github.com/repos/$script:GitHubRepo/releases/latest"
    $ReleasesTagUrl = "https://api.github.com/repos/$script:GitHubRepo/releases/tags"
    $DownloadBaseUrl = "https://github.com/$script:GitHubRepo/releases/download"

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
            Write-Log "  ${Bold}curl -fsSL https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.sh | sh${Reset}"
            Write-Log ""
            throw 'This installer is for Windows only.'
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
            if (-not (Test-VersionFormat -Version $version)) {
                Stop-WithError @"
Invalid INVOWK_VERSION: $version
Use a release tag such as v1.0.0 or v0.1.0-alpha.1 (the v prefix is optional).
Check available versions at: https://github.com/$script:GitHubRepo/releases
"@
            }
            Write-Log "Using specified version: ${Bold}${version}${Reset}"

            # Verify the release exists by checking the tag endpoint.
            try {
                $null = Invoke-SafeWebRequest -Uri "$ReleasesTagUrl/$version"
            }
            catch {
                Stop-WithError @"
Version $version not found.
Check available versions at: https://github.com/$script:GitHubRepo/releases
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
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.ps1 | iex

Or set a GitHub token for higher rate limits:
  `$env:GITHUB_TOKEN='ghp_...'; irm https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.ps1 | iex
"@
            }
            Stop-WithError @"
Failed to fetch latest release from GitHub.

Check your network connection and try again.
If behind a firewall, specify a version directly:
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.ps1 | iex
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
  `$env:INVOWK_VERSION='v1.0.0'; irm https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.ps1 | iex
"@
            }
            Stop-WithError 'Could not determine latest version from GitHub API response.'
        }

        # Validate the extracted version looks like a semver tag (vN.N.N with optional suffix).
        if (-not (Test-VersionFormat -Version $version)) {
            Stop-WithError @"
Unexpected version format from GitHub API: $version
This may indicate an API change. Report at: https://github.com/$script:GitHubRepo/issues
"@
        }

        Write-Log "Latest stable version: ${Bold}${version}${Reset}"
        return $version
    }

    # -------------------------------------------------------------------------
    # Installation
    # -------------------------------------------------------------------------

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
            Write-Log "Downloading ${Bold}${script:BinaryName} ${Version}${Reset} for windows/${Arch}..."
            try {
                Invoke-SafeWebRequest -Uri $downloadUrl -OutFile $archivePath
            }
            catch {
                Stop-WithError @"
Failed to download $asset.

The release asset may not exist for your platform (windows/$Arch).
Check available assets at: https://github.com/$script:GitHubRepo/releases/tag/$Version
"@
            }

            # The checksum file and its signature are downloaded to a freshly
            # created temp directory before verification.
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

            # checksums.txt is trusted only after its signature verifies (when cosign
            # is available; see Test-ChecksumsSignature).
            try {
                Test-ChecksumsSignature `
                    -Version $Version `
                    -ChecksumsPath $checksumsPath `
                    -DownloadBaseUrl $DownloadBaseUrl `
                    -DownloadFile { param($Url, $Dest) Invoke-SafeWebRequest -Uri $Url -OutFile $Dest } `
                    -WriteInfo { param($Msg) Write-Log $Msg } `
                    -WriteSuccess { param($Msg) Write-Ok $Msg } `
                    -WriteWarning { param($Msg) Write-Warn $Msg }
            }
            catch {
                Stop-WithError $_.Exception.Message
            }

            # Extract expected checksum for our asset from checksums.txt.
            # Use exact field match (not substring) to avoid matching .sig or similar suffixed entries.
            $checksumLine = Get-Content $checksumsPath | Where-Object { ($_ -split '\s+')[1] -eq $asset }
            if (-not $checksumLine) {
                Stop-WithError @"
Asset $asset not found in checksums.txt.
This may indicate a GoReleaser configuration issue.
Report at: https://github.com/$script:GitHubRepo/issues
"@
            }
            $expectedHash = ($checksumLine -split '\s+')[0]
            if (-not $expectedHash) {
                Stop-WithError "Failed to extract checksum hash for '$asset' from checksums file"
            }

            # Verify the archive checksum.
            Write-Log 'Verifying checksum...'
            Test-Checksum -FilePath $archivePath -Expected $expectedHash
            Write-Ok 'Checksum verified.'

            # Extract the archive.
            Write-Log 'Extracting binary...'
            $extractDir = Join-Path $tempDir 'extracted'
            try {
                Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force
            }
            catch {
                Stop-WithError @"
Failed to extract $asset.

The archive may be corrupted or the disk may be full.
Try downloading again, or report at: https://github.com/$script:GitHubRepo/issues
"@
            }

            $binaryPath = Join-Path $extractDir "${script:BinaryName}.exe"
            if (-not (Test-Path $binaryPath)) {
                Stop-WithError "Binary '${script:BinaryName}.exe' not found in archive $asset."
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
            $destPath = Join-Path $InstallDir "${script:BinaryName}.exe"
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
  `$env:INSTALL_DIR='C:\tools\invowk'; irm https://raw.githubusercontent.com/$script:GitHubRepo/main/scripts/install.ps1 | iex
"@
            }

            Write-Ok "Successfully installed ${Bold}${script:BinaryName} ${Version}${Reset} to $destPath"
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

        if (Test-InPath -Dir $InstallDir -PathString $currentPath) {
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

    Write-Log "Installing ${Bold}${script:BinaryName}${Reset}..."
    Write-Log ''

    # Reject an invalid INVOWK_REQUIRE_SIGNATURE before anything is downloaded.
    try {
        $null = Test-SignatureRequired
    }
    catch {
        Stop-WithError $_.Exception.Message
    }

    Test-Prerequisites

    $arch = Get-Architecture

    # Resolve installation directory.
    # $env:LOCALAPPDATA may be empty on Windows Server Core or certain CI environments.
    $localAppData = $env:LOCALAPPDATA
    if (-not $localAppData) {
        $localAppData = Join-Path $env:USERPROFILE 'AppData\Local'
    }
    $installDir = if ($env:INSTALL_DIR) {
        $env:INSTALL_DIR
    } else {
        Join-Path $localAppData 'Programs\invowk'
    }

    # Resolve target version.
    $version = Resolve-Version
    if (-not $version) {
        Stop-WithError 'Failed to resolve version. Set INVOWK_VERSION explicitly or check network connectivity.'
    }

    # Perform installation.
    Install-Binary -Version $version -Arch $arch -InstallDir $installDir

    # PATH management.
    Update-Path -InstallDir $installDir

    # Verify installation.
    $installedBinary = Join-Path $installDir "${script:BinaryName}.exe"
    if (Test-Path $installedBinary) {
        try {
            $installedVersion = & $installedBinary --version 2>&1
            Write-Ok ''
            Write-Ok "Verify: ${Bold}${script:BinaryName} --version${Reset} -> $installedVersion"
        }
        catch {
            Write-Log ''
            Write-Log "Run '${Bold}${script:BinaryName} --version${Reset}' to verify the installation."
        }
    }
}

# Allow dot-sourcing for testing without executing the installer.
if ($env:INVOWK_INSTALL_TESTING -ne '1') {
    Install-Invowk
}
