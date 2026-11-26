# SPDX-License-Identifier: MPL-2.0
#
# Unit tests for install.ps1 pure functions.
# Usage: pwsh scripts/test_install.ps1       (PowerShell 7+)
#        powershell scripts/test_install.ps1  (Windows PowerShell 5.1)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Dot-source install.ps1 without executing the installer.
$env:INVOWK_INSTALL_TESTING = '1'
. (Join-Path $ScriptDir 'install.ps1')

# -------------------------------------------------------------------------
# Test Helpers
# -------------------------------------------------------------------------

$script:Pass = 0
$script:Fail = 0

function Assert-Equal {
    param(
        [string]$Description,
        [string]$Expected,
        [string]$Actual
    )
    if ($Expected -eq $Actual) {
        $script:Pass++
    } else {
        $script:Fail++
        Write-Host "FAIL: $Description"
        Write-Host "  expected: $Expected"
        Write-Host "  actual:   $Actual"
    }
}

function Assert-True {
    param(
        [string]$Description,
        [bool]$Value
    )
    if ($Value) {
        $script:Pass++
    } else {
        $script:Fail++
        Write-Host "FAIL: $Description (expected True, got False)"
    }
}

function Assert-False {
    param(
        [string]$Description,
        [bool]$Value
    )
    if (-not $Value) {
        $script:Pass++
    } else {
        $script:Fail++
        Write-Host "FAIL: $Description (expected False, got True)"
    }
}

function Assert-Throws {
    param(
        [string]$Description,
        [scriptblock]$ScriptBlock,
        [string]$MessageContains
    )
    $threw = $false
    $errorMsg = ''
    try {
        & $ScriptBlock
    } catch {
        $threw = $true
        $errorMsg = $_.Exception.Message
    }
    if (-not $threw) {
        $script:Fail++
        Write-Host "FAIL: $Description (expected throw, but no exception was thrown)"
        return
    }
    if ($MessageContains -and $errorMsg -notlike "*$MessageContains*") {
        $script:Fail++
        Write-Host "FAIL: $Description"
        Write-Host "  expected message containing: $MessageContains"
        Write-Host "  actual message: $errorMsg"
        return
    }
    $script:Pass++
}

# -------------------------------------------------------------------------
# Tests: Get-AssetFilename
# -------------------------------------------------------------------------

Assert-Equal 'Get-AssetFilename windows amd64' `
    'invowk_1.0.0_windows_amd64.zip' `
    (Get-AssetFilename -Version 'v1.0.0' -Arch 'amd64')

Assert-Equal 'Get-AssetFilename strips v prefix' `
    'invowk_2.3.4_windows_amd64.zip' `
    (Get-AssetFilename -Version 'v2.3.4' -Arch 'amd64')

Assert-Equal 'Get-AssetFilename prerelease version' `
    'invowk_1.0.0-alpha.1_windows_amd64.zip' `
    (Get-AssetFilename -Version 'v1.0.0-alpha.1' -Arch 'amd64')

Assert-Equal 'Get-AssetFilename without v prefix input' `
    'invowk_3.0.0_windows_amd64.zip' `
    (Get-AssetFilename -Version '3.0.0' -Arch 'amd64')

# -------------------------------------------------------------------------
# Tests: Get-Architecture
# -------------------------------------------------------------------------

# Save original environment values.
$savedArch = $env:PROCESSOR_ARCHITECTURE
$savedW6432 = $env:PROCESSOR_ARCHITEW6432

# Test AMD64 architecture.
$env:PROCESSOR_ARCHITECTURE = 'AMD64'
$env:PROCESSOR_ARCHITEW6432 = $null
Assert-Equal 'Get-Architecture AMD64' 'amd64' (Get-Architecture)

# Test 32-on-64 override (PROCESSOR_ARCHITEW6432 takes precedence).
$env:PROCESSOR_ARCHITECTURE = 'x86'
$env:PROCESSOR_ARCHITEW6432 = 'AMD64'
Assert-Equal 'Get-Architecture 32-on-64 override' 'amd64' (Get-Architecture)

# Test unsupported x86 (no W6432 override).
$env:PROCESSOR_ARCHITECTURE = 'x86'
$env:PROCESSOR_ARCHITEW6432 = $null
Assert-Throws 'Get-Architecture rejects x86' `
    { Get-Architecture } `
    '32-bit Windows is not supported'

# Test unsupported ARM64.
$env:PROCESSOR_ARCHITECTURE = 'ARM64'
$env:PROCESSOR_ARCHITEW6432 = $null
Assert-Throws 'Get-Architecture rejects ARM64' `
    { Get-Architecture } `
    'ARM64 is not currently supported'

# Restore environment.
$env:PROCESSOR_ARCHITECTURE = $savedArch
$env:PROCESSOR_ARCHITEW6432 = $savedW6432

# -------------------------------------------------------------------------
# Tests: Test-Checksum
# -------------------------------------------------------------------------

# Create a temp file with known content.
$tempFile = [IO.Path]::GetTempFileName()
try {
    # Write 'hello' (UTF-8 no BOM) to the temp file.
    [IO.File]::WriteAllText($tempFile, 'hello')
    $expectedHash = (Get-FileHash -Path $tempFile -Algorithm SHA256).Hash.ToLower()

    # Correct checksum should not throw.
    Assert-Throws 'Test-Checksum passes with correct hash' `
        { Test-Checksum -FilePath $tempFile -Expected $expectedHash; throw 'NO_THROW_SENTINEL' } `
        'NO_THROW_SENTINEL'

    # Case-insensitive: uppercase expected should also pass.
    Assert-Throws 'Test-Checksum case-insensitive' `
        { Test-Checksum -FilePath $tempFile -Expected $expectedHash.ToUpper(); throw 'NO_THROW_SENTINEL' } `
        'NO_THROW_SENTINEL'

    # Wrong checksum should throw.
    Assert-Throws 'Test-Checksum rejects wrong hash' `
        { Test-Checksum -FilePath $tempFile -Expected 'deadbeef' } `
        'Checksum verification failed'
} finally {
    Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
}

# -------------------------------------------------------------------------
# Tests: Test-InPath
# -------------------------------------------------------------------------

Assert-True 'Test-InPath finds existing entry' `
    (Test-InPath -Dir 'C:\foo' -PathString 'C:\bar;C:\foo;C:\baz')

Assert-False 'Test-InPath rejects missing entry' `
    (Test-InPath -Dir 'C:\missing' -PathString 'C:\bar;C:\foo;C:\baz')

Assert-True 'Test-InPath trailing backslash normalization' `
    (Test-InPath -Dir 'C:\foo\' -PathString 'C:\bar;C:\foo;C:\baz')

Assert-True 'Test-InPath path entry has trailing backslash' `
    (Test-InPath -Dir 'C:\foo' -PathString 'C:\bar;C:\foo\;C:\baz')

Assert-False 'Test-InPath rejects substring match' `
    (Test-InPath -Dir 'C:\foo' -PathString 'C:\foobar;C:\baz')

Assert-False 'Test-InPath empty path string' `
    (Test-InPath -Dir 'C:\foo' -PathString '')

# -------------------------------------------------------------------------
# Tests: Host-dependent (verify Get-Architecture on Windows CI runner)
# -------------------------------------------------------------------------

# Restore real environment for host detection.
$env:PROCESSOR_ARCHITECTURE = $savedArch
$env:PROCESSOR_ARCHITEW6432 = $savedW6432

if ($PSVersionTable.PSEdition -eq 'Desktop' -or $IsWindows) {
    $hostArch = Get-Architecture
    # CI runners (windows-latest) are amd64.
    if ($hostArch -eq 'amd64') {
        $script:Pass++
    } else {
        $script:Fail++
        Write-Host "FAIL: Get-Architecture on host returned unexpected value: $hostArch"
    }
}

# -------------------------------------------------------------------------
# Tests: Test-VersionFormat & Get-SignerIdentity
# -------------------------------------------------------------------------

foreach ($validTag in @('v1.0.0', 'v0.1.0-alpha.1', 'v1.2.3+build.4')) {
    Assert-True "Test-VersionFormat accepts '$validTag'" (Test-VersionFormat -Version $validTag)
}
foreach ($badTag in @('', 'latest', 'v1.2', '1', 'v', '../v1.0.0', 'vx.y.z', 'v1.0.0/../../evil/v1.0.0', 'v1.0.0 x', 'v1.0.0;x', 'v1.0.0\foo', "v1.0.0`n", "v1.0.0`r`n")) {
    Assert-False "Test-VersionFormat rejects '$badTag'" (Test-VersionFormat -Version $badTag)
}

Assert-Equal 'Get-SignerIdentity pins release.yml at the version tag' `
    'https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v1.2.3' `
    (Get-SignerIdentity -Version 'v1.2.3')

# -------------------------------------------------------------------------
# Tests: Test-SignatureRequired
# -------------------------------------------------------------------------

$savedRequireSig = $env:INVOWK_REQUIRE_SIGNATURE
try {
    foreach ($val in @('1', 'true', 'TRUE', 'True', 'yes', 'YES', 'Yes')) {
        $env:INVOWK_REQUIRE_SIGNATURE = $val
        Assert-True "Test-SignatureRequired treats '$val' as true" (Test-SignatureRequired)
    }
    foreach ($val in @('', '0', 'false', 'FALSE', 'no', 'NO', 'No')) {
        $env:INVOWK_REQUIRE_SIGNATURE = $val
        Assert-False "Test-SignatureRequired treats '$val' as false" (Test-SignatureRequired)
    }
    $env:INVOWK_REQUIRE_SIGNATURE = $null
    Assert-False 'Test-SignatureRequired treats unset as false' (Test-SignatureRequired)

    foreach ($val in @('2', 'on', 'off', 'y', 'n', 'required', ' 1', 'true ')) {
        $env:INVOWK_REQUIRE_SIGNATURE = $val
        Assert-Throws "Test-SignatureRequired rejects '$val'" `
            { Test-SignatureRequired } `
            "Invalid INVOWK_REQUIRE_SIGNATURE value: $val"
    }
} finally {
    $env:INVOWK_REQUIRE_SIGNATURE = $savedRequireSig
}

# -------------------------------------------------------------------------
# Tests: Test-VersionAtLeast & Get-CosignVersion
# -------------------------------------------------------------------------

Assert-True 'Test-VersionAtLeast accepts the same version' (Test-VersionAtLeast -Have '2.4.2' -Min '2.4.2')
Assert-False 'Test-VersionAtLeast rejects an older patch' (Test-VersionAtLeast -Have '2.4.1' -Min '2.4.2')
Assert-False 'Test-VersionAtLeast rejects an older minor' (Test-VersionAtLeast -Have '2.3.9' -Min '2.4.2')
Assert-False 'Test-VersionAtLeast rejects an older major' (Test-VersionAtLeast -Have '1.99.99' -Min '2.4.2')
Assert-True 'Test-VersionAtLeast accepts a newer patch' (Test-VersionAtLeast -Have '2.4.3' -Min '2.4.2')
Assert-True 'Test-VersionAtLeast compares numerically' (Test-VersionAtLeast -Have '2.10.0' -Min '2.4.2')
Assert-True 'Test-VersionAtLeast accepts a newer major' (Test-VersionAtLeast -Have '3.0.0' -Min '2.4.2')
Assert-False 'Test-VersionAtLeast rejects trailing newline' (Test-VersionAtLeast -Have "2.4.2`n" -Min '2.4.2')

$cosignBanner = @"
  ______   ______
 cosign: A tool for Container Signing, Verification and Storage in an OCI registry.

GitVersion:    v3.1.3
GitCommit:     unknown
GitTreeState:  clean
"@
Assert-Equal 'Get-CosignVersion reads the GitVersion line' '3.1.3' (Get-CosignVersion -Output $cosignBanner)
Assert-Equal 'Get-CosignVersion handles CRLF banners' '3.1.4' (Get-CosignVersion -Output ($cosignBanner -replace 'v3\.1\.3', 'v3.1.4' -replace "`n", "`r`n"))
Assert-Equal 'Get-CosignVersion accepts no v prefix' '2.4.2' (Get-CosignVersion -Output 'GitVersion: 2.4.2')
Assert-Equal 'Get-CosignVersion drops a prerelease suffix' '2.5.0' (Get-CosignVersion -Output 'GitVersion:    v2.5.0-rc.1')
Assert-Equal 'Get-CosignVersion returns empty for a development build' '' (Get-CosignVersion -Output 'GitVersion:    devel')
Assert-Equal 'Get-CosignVersion returns empty without a GitVersion line' '' (Get-CosignVersion -Output 'cosign version 3.1.3')

# -------------------------------------------------------------------------
# Tests: Test-CosignOutputIsNetworkError
# -------------------------------------------------------------------------

Assert-True 'Test-CosignOutputIsNetworkError detects TUF fetch failure' `
    (Test-CosignOutputIsNetworkError -Output 'Error: setting trusted material: getting trusted root from TUF for new bundle verification: error getting live trusted root: failed to create TUF client failed to load metadata: tuf refresh failed: Get "https://tuf-repo-cdn.sigstore.dev/15.root.json": proxyconnect tcp: dial tcp 127.0.0.1:9: connect: connection refused')
Assert-True 'Test-CosignOutputIsNetworkError detects DNS failure' `
    (Test-CosignOutputIsNetworkError -Output 'error during command execution: getting rekor public keys: Get "https://rekor.sigstore.dev/api/v1/log/publicKey": dial tcp: lookup rekor.sigstore.dev on 127.0.0.53:53: no such host')
Assert-False 'Test-CosignOutputIsNetworkError keeps a bad signature a bad signature' `
    (Test-CosignOutputIsNetworkError -Output 'Error: failed to verify signature: could not verify message: invalid signature when validating ASN.1 encoded signature')
Assert-False 'Test-CosignOutputIsNetworkError keeps a wrong identity a bad signature' `
    (Test-CosignOutputIsNetworkError -Output 'Error: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN value "https://github.com/invowk/invowk/.github/workflows/ci.yml@refs/tags/v0.17.0", got "https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v0.17.0"')
Assert-False 'Test-CosignOutputIsNetworkError keeps attacker SAN mentioning tuf a bad signature' `
    (Test-CosignOutputIsNetworkError -Output 'Error: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN value "https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v0.17.0", got "https://github.com/attacker/fork/.github/workflows/tuf.yml@refs/tags/v0.17.0"')

# -------------------------------------------------------------------------
# Tests: Test-ChecksumsSignature & Invoke-CosignCommand
# -------------------------------------------------------------------------

$sigTmpDir = Join-Path ([IO.Path]::GetTempPath()) ("invowk-sig-test-" + [IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $sigTmpDir -Force | Out-Null
$savedReqForSig = $env:INVOWK_REQUIRE_SIGNATURE
try {
    $checksumsFile = Join-Path $sigTmpDir 'checksums.txt'
    [IO.File]::WriteAllText($checksumsFile, "deadbeef  invowk_1.2.3_windows_amd64.zip`n")

    $script:FakeCosignVersionOut = 'GitVersion:    v3.1.4'
    $script:FakeCosignVersionRc = 0
    $script:FakeCosignVerifyOut = ''
    $script:FakeCosignVerifyRc = 0
    $script:FakeSigAssets = @()
    $script:FakeCosignArgs = $null
    $script:FakeCosignCalls = @()
    $script:SigInfos = @()
    $script:SigSuccesses = @()
    $script:SigWarnings = @()

    $fakeDownload = {
        param($Url, $Dest)
        foreach ($suffix in $script:FakeSigAssets) {
            if ($Url.EndsWith($suffix)) {
                [IO.File]::WriteAllText($Dest, "sig`n")
                return
            }
        }
        throw "404 Not Found: $Url"
    }

    $fakeRunCosign = {
        param($Exe, [string[]]$CmdArgs)
        if ($CmdArgs.Count -gt 0 -and $CmdArgs[0] -eq 'version') {
            return @{ ExitCode = $script:FakeCosignVersionRc; Output = $script:FakeCosignVersionOut }
        }
        $script:FakeCosignArgs = $CmdArgs
        $script:FakeCosignCalls += $CmdArgs[0]
        return @{ ExitCode = $script:FakeCosignVerifyRc; Output = $script:FakeCosignVerifyOut }
    }

    function Invoke-TestChecksumsSig {
        param(
            [string]$RequireSig,
            [bool]$HasCosign,
            [string[]]$Assets,
            [int]$VerifyRc = 0,
            [string]$VerifyOut = '',
            [string]$VersionOut = 'GitVersion:    v3.1.4',
            [int]$VersionRc = 0
        )
        foreach ($ext in @('.sigstore.json', '.sig', '.pem')) {
            Remove-Item -LiteralPath "$checksumsFile$ext" -Force -ErrorAction SilentlyContinue
        }
        $script:FakeCosignArgs = $null
        $script:FakeCosignCalls = @()
        $script:SigInfos = @()
        $script:SigSuccesses = @()
        $script:SigWarnings = @()
        $script:FakeSigAssets = $Assets
        $script:FakeCosignVerifyRc = $VerifyRc
        $script:FakeCosignVerifyOut = $VerifyOut
        $script:FakeCosignVersionOut = $VersionOut
        $script:FakeCosignVersionRc = $VersionRc
        $env:INVOWK_REQUIRE_SIGNATURE = $RequireSig

        $findCb = if ($HasCosign) { { 'C:\fake\cosign.exe' } } else { { $null } }
        Test-ChecksumsSignature `
            -Version 'v1.2.3' `
            -ChecksumsPath $checksumsFile `
            -FindCosign $findCb `
            -DownloadFile $fakeDownload `
            -RunCosign $fakeRunCosign `
            -WriteInfo { param($m) $script:SigInfos += $m } `
            -WriteSuccess { param($m) $script:SigSuccesses += $m } `
            -WriteWarning { param($m) $script:SigWarnings += $m }
    }

    # Without cosign: unset / 0 / false / No warn and continue; 1 / true / YES throw.
    Invoke-TestChecksumsSig -RequireSig $null -HasCosign $false -Assets @('.sigstore.json')
    Assert-True 'Missing cosign warns with minimum version hint' `
        (($script:SigWarnings -join "`n") -like '*Install cosign v2.4.2 or later*')

    foreach ($v in @('0', 'false', 'No')) {
        Invoke-TestChecksumsSig -RequireSig $v -HasCosign $false -Assets @('.sigstore.json')
        Assert-True "Missing cosign continues when INVOWK_REQUIRE_SIGNATURE=$v" ($script:SigWarnings.Count -gt 0)
    }

    foreach ($v in @('1', 'true', 'YES')) {
        Assert-Throws "Missing cosign throws when INVOWK_REQUIRE_SIGNATURE=$v" `
            { Invoke-TestChecksumsSig -RequireSig $v -HasCosign $false -Assets @('.sigstore.json') } `
            "INVOWK_REQUIRE_SIGNATURE=$v"
    }

    # Invalid INVOWK_REQUIRE_SIGNATURE fails fast even with cosign present, never calling verify-blob.
    Assert-Throws 'Invalid INVOWK_REQUIRE_SIGNATURE rejected without cosign' `
        { Invoke-TestChecksumsSig -RequireSig 'maybe' -HasCosign $false -Assets @('.sigstore.json') } `
        'Invalid INVOWK_REQUIRE_SIGNATURE value: maybe'
    Assert-Throws 'Invalid INVOWK_REQUIRE_SIGNATURE rejected with cosign without calling cosign' `
        { Invoke-TestChecksumsSig -RequireSig 'on' -HasCosign $true -Assets @('.sigstore.json') } `
        'Invalid INVOWK_REQUIRE_SIGNATURE value: on'
    Assert-True 'Cosign not invoked on invalid INVOWK_REQUIRE_SIGNATURE' ($null -eq $script:FakeCosignArgs)

    # Verified bundle asserts exact pinned argument vector.
    Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json')
    $expectedBundleArgs = @(
        'verify-blob',
        '--bundle', "$checksumsFile.sigstore.json",
        '--certificate-identity', 'https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v1.2.3',
        '--certificate-oidc-issuer', 'https://token.actions.githubusercontent.com',
        '--certificate-github-workflow-trigger', 'push',
        $checksumsFile
    )
    Assert-Equal 'Bundle verification pins identity, issuer and trigger' `
        ($expectedBundleArgs -join '|') ($script:FakeCosignArgs -join '|')
    Assert-True 'Verified bundle prints success message' (($script:SigSuccesses -join "`n") -eq 'Signature of checksums.txt verified.')

    # Bad signature fails closed.
    Assert-Throws 'Bad signature fails closed' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VerifyRc 1 -VerifyOut 'Error: failed to verify signature: invalid signature' } `
        'Signature verification of checksums.txt failed for v1.2.3.'
    Assert-Throws 'Bad signature includes Do not install guidance' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VerifyRc 1 -VerifyOut 'invalid signature' } `
        'Do not install this release.'

    # Network error fails closed with network wording (and NOT bad-signature wording).
    $netErr = ''
    try {
        Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VerifyRc 1 -VerifyOut 'Error: tuf refresh failed: connection refused'
    } catch {
        $netErr = $_.Exception.Message
    }
    Assert-True 'Network failure names Sigstore services' ($netErr -like '*could not reach the Sigstore services*')
    Assert-True 'Network failure says Do not install' ($netErr -like '*Do not install this release*')
    Assert-False 'Network failure does not call it bad signature' ($netErr -like '*Signature verification of checksums.txt failed*')

    # Attacker SAN mentioning tuf remains bad signature.
    Assert-Throws 'Attacker SAN mentioning tuf fails as bad signature' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VerifyRc 1 -VerifyOut 'no matching CertificateIdentity found, expected SAN "release.yml", got "https://github.com/attacker/fork/.github/workflows/tuf.yml@refs/tags/v0.17.0"' } `
        'Signature verification of checksums.txt failed for v1.2.3.'

    # Old cosign (< 2.4.2) rejected for bundle without saying Do not install and without running verify-blob.
    $oldErr = ''
    try {
        Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VersionOut 'GitVersion:    v2.4.1'
    } catch {
        $oldErr = $_.Exception.Message
    }
    Assert-True 'Old cosign mentions minimum 2.4.2' ($oldErr -like '*older than the minimum 2.4.2*')
    Assert-False 'Old cosign does not blame the release' ($oldErr -like '*Do not install this release.*')
    Assert-True 'Old cosign does not invoke verify-blob' ($null -eq $script:FakeCosignArgs)

    # Old cosign still verifies legacy .sig/.pem releases.
    Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sig', '.pem') -VersionOut 'GitVersion:    v2.4.1'
    $expectedLegacyArgs = @(
        'verify-blob',
        '--signature', "$checksumsFile.sig",
        '--certificate', "$checksumsFile.pem",
        '--certificate-identity', 'https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v1.2.3',
        '--certificate-oidc-issuer', 'https://token.actions.githubusercontent.com',
        '--certificate-github-workflow-trigger', 'push',
        $checksumsFile
    )
    Assert-Equal 'Legacy signature pins identity, issuer and trigger' `
        ($expectedLegacyArgs -join '|') ($script:FakeCosignArgs -join '|')

    # Supported cosign versions pass for bundles.
    foreach ($vOut in @('GitVersion:    v2.4.2', 'GitVersion: 2.10.0', 'GitVersion:    v3.0.1')) {
        Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VersionOut $vOut
        Assert-Equal "Accepted bundle with '$vOut'" 'verify-blob' ($script:FakeCosignCalls -join ',')
    }

    # Unreadable cosign version warns and still runs verify-blob (and fails closed if verify-blob fails).
    Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VersionOut 'GitVersion:    devel'
    Assert-True 'Unreadable cosign version warns' (($script:SigWarnings -join "`n") -like '*Could not read the cosign version*')
    Assert-Throws 'Unreadable cosign version still fails closed on bad verify' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VersionOut 'GitVersion:    devel' -VerifyRc 1 -VerifyOut 'bad' } `
        'Signature verification of checksums.txt failed'

    # Broken cosign version (rc != 0) stops before verify-blob.
    Assert-Throws 'Broken cosign version fails clearly' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json') -VersionRc 1 -VersionOut 'boom' } `
        "'cosign version' failed"
    Assert-True 'Broken cosign does not call verify-blob' ($null -eq $script:FakeCosignArgs)

    # Both bundle and legacy exist: bundle wins and failure never falls back to legacy.
    Assert-Throws 'Bundle failure never falls back to legacy .sig/.pem' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sigstore.json', '.sig', '.pem') -VerifyRc 1 -VerifyOut 'invalid signature' } `
        'Signature verification of checksums.txt failed'
    Assert-Equal 'Only one verify-blob call made when bundle fails' 'verify-blob' ($script:FakeCosignCalls -join ',')
    Assert-Equal 'Failing verify-blob call used bundle' ($expectedBundleArgs -join '|') ($script:FakeCosignArgs -join '|')

    # No signature assets at all, or partial legacy pair (.sig without .pem), fails closed without invoking cosign.
    Assert-Throws 'Missing signature assets fails closed' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @() } `
        'Could not download the signature of checksums.txt for v1.2.3.'
    Assert-True 'Missing signature assets does not invoke verify-blob' ($null -eq $script:FakeCosignArgs)

    Assert-Throws 'Partial legacy signature (.sig without .pem) fails closed' `
        { Invoke-TestChecksumsSig -RequireSig '' -HasCosign $true -Assets @('.sig') } `
        'Could not download the signature of checksums.txt for v1.2.3.'
    Assert-False 'Partial legacy failure cleans up .sig' (Test-Path -LiteralPath "$checksumsFile.sig")
    Assert-True 'Partial legacy signature does not invoke verify-blob' ($null -eq $script:FakeCosignArgs)

    # Exercise Invoke-CosignCommand directly using pwsh/powershell process (non-zero exit + stderr) to verify ErrorActionPreference=Stop handling.
    $probeExe = [System.Diagnostics.Process]::GetCurrentProcess().MainModule.FileName
    $okRes = Invoke-CosignCommand -CosignPath $probeExe -Arguments @('-NoProfile', '-Command', "Write-Output 'ok-line'; exit 0")
    Assert-Equal 'Invoke-CosignCommand captures exit code 0' '0' "$($okRes.ExitCode)"
    Assert-True 'Invoke-CosignCommand captures stdout' ($okRes.Output -like '*ok-line*')
    $errRes = Invoke-CosignCommand -CosignPath $probeExe -Arguments @('-NoProfile', '-Command', "[Console]::Error.WriteLine('err-line'); exit 2")
    Assert-Equal 'Invoke-CosignCommand captures non-zero exit code without throwing' '2' "$($errRes.ExitCode)"
    Assert-True 'Invoke-CosignCommand captures stderr' ($errRes.Output -like '*err-line*')
} finally {
    $env:INVOWK_REQUIRE_SIGNATURE = $savedReqForSig
    Remove-Item -LiteralPath $sigTmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

# -------------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------------

Write-Host ''
Write-Host "$($script:Pass) passed, $($script:Fail) failed"

if ($script:Fail -gt 0) {
    exit 1
}
