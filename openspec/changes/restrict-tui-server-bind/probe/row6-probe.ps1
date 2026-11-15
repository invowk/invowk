# SPDX-License-Identifier: MPL-2.0
#
# Docker Desktop row-6 probe for restrict-tui-server-bind (design.md D12),
# for native Windows (Windows PowerShell 5.1 or PowerShell 7). It needs only
# the docker CLI with Docker Desktop in Linux-container mode; no invowk build.
#
# It starts two listeners on this machine: 127.0.0.1 (what row 6 binds) and
# 0.0.0.0 (the opt-in, as a control). Each one writes a random token to every
# connection. A debian:stable-slim container then dials host.docker.internal
# with the host mapping invowk adds, and reports a listener as reachable only
# when it reads that token back, so a proxy that merely accepts the
# connection does not count.
#
# Usage: powershell -ExecutionPolicy Bypass -File row6-probe.ps1
$ErrorActionPreference = 'Stop'

$image = 'debian:stable-slim'
$token = 'invowk-row6-' + [guid]::NewGuid().ToString('N')

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Error 'row6-probe: the docker CLI is required'
    exit 2
}

$loopback = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$wildcard = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Any, 0)
$loopback.Start()
$wildcard.Start()
$loopbackPort = ([System.Net.IPEndPoint]$loopback.LocalEndpoint).Port
$wildcardPort = ([System.Net.IPEndPoint]$wildcard.LocalEndpoint).Port

# The container script arrives on stdin, so no argument needs shell quoting.
$containerScript = @'
probe() {
	local line=""
	if { exec 3<>"/dev/tcp/host.docker.internal/$2"; } 2>/dev/null; then
		read -r -t 5 line <&3 || true
		exec 3<&-
	fi
	line=${line%$'\r'}
	if [ "$line" = "$TOKEN" ]; then echo "$1: reachable"; else echo "$1: unreachable"; fi
}
probe "loopback (row 6 plan)" "$LOOPBACK_PORT"
probe "all-interfaces (opt-in control)" "$WILDCARD_PORT"
'@ -replace "`r", ''

try {
    $engine = & docker info --format '{{.OperatingSystem}} / server {{.ServerVersion}}' 2>&1
    Write-Output "engine: $engine"

    $psi = [System.Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = (Get-Command docker).Source
    $psi.Arguments = "run --rm -i --add-host host.docker.internal:host-gateway " +
        "-e TOKEN=$token -e LOOPBACK_PORT=$loopbackPort -e WILDCARD_PORT=$wildcardPort " +
        "$image bash -s"
    $psi.UseShellExecute = $false
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $proc = [System.Diagnostics.Process]::Start($psi)
    $stdout = $proc.StandardOutput.ReadToEndAsync()
    $stderr = $proc.StandardError.ReadToEndAsync()
    $proc.StandardInput.NewLine = "`n"
    $proc.StandardInput.Write($containerScript)
    $proc.StandardInput.Close()

    # Serve both listeners until the container exits (image pulls included).
    $deadline = (Get-Date).AddMinutes(5)
    while (-not $proc.HasExited -and (Get-Date) -lt $deadline) {
        foreach ($listener in @($loopback, $wildcard)) {
            while ($listener.Pending()) {
                $client = $listener.AcceptTcpClient()
                $bytes = [System.Text.Encoding]::ASCII.GetBytes("$token`n")
                $stream = $client.GetStream()
                $stream.Write($bytes, 0, $bytes.Length)
                $stream.Flush()
                $client.Close()
            }
        }
        Start-Sleep -Milliseconds 50
    }
    if (-not $proc.HasExited) {
        $proc.Kill()
        Write-Error 'row6-probe: the container did not finish within 5 minutes'
        exit 2
    }
    Write-Output $stdout.Result.TrimEnd()
    if ($proc.ExitCode -ne 0) {
        Write-Output $stderr.Result.TrimEnd()
        exit $proc.ExitCode
    }
}
finally {
    $loopback.Stop()
    $wildcard.Stop()
}
