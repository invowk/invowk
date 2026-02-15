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
# Tests: Host-dependent (verify Get-Architecture on the CI runner)
# -------------------------------------------------------------------------

# Restore real environment for host detection.
$env:PROCESSOR_ARCHITECTURE = $savedArch
$env:PROCESSOR_ARCHITEW6432 = $savedW6432

$hostArch = Get-Architecture
# CI runners (windows-latest) are amd64.
if ($hostArch -eq 'amd64') {
    $script:Pass++
} else {
    $script:Fail++
    Write-Host "FAIL: Get-Architecture on host returned unexpected value: $hostArch"
}

# -------------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------------

Write-Host ''
Write-Host "$($script:Pass) passed, $($script:Fail) failed"

if ($script:Fail -gt 0) {
    exit 1
}
