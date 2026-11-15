#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Docker Desktop row-6 probe for restrict-tui-server-bind (design.md D12).
# Run it where invowk would run: inside the WSL2 distribution that uses Docker
# Desktop's WSL integration, or in a macOS terminal. It needs bash, perl
# (perl-base on Debian/Ubuntu) and the docker CLI; no invowk build.
#
# It starts two listeners on this machine: 127.0.0.1 (what row 6 binds) and
# 0.0.0.0 (the opt-in, as a control). Each one writes a random token to every
# connection. A debian:stable-slim container then dials host.docker.internal
# with the host mapping invowk adds, and reports a listener as reachable only
# when it reads that token back, so a proxy that merely accepts the
# connection does not count.
#
# Usage: bash row6-probe.sh
set -euo pipefail

image=debian:stable-slim
token="invowk-row6-$RANDOM$RANDOM$RANDOM"
workdir=$(mktemp -d)
pids=()
cleanup() {
	for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
	rm -rf "$workdir"
}
trap cleanup EXIT

for tool in docker perl; do
	command -v "$tool" >/dev/null || { echo "row6-probe: $tool is required" >&2; exit 2; }
done

# start_listener ADDR NAME: listen on ADDR at a free port; the port lands in
# $workdir/NAME.port. The listener exits on its own after 120 s.
start_listener() {
	perl -MIO::Socket::INET -e '
		my ($addr, $token, $portfile) = @ARGV;
		my $srv = IO::Socket::INET->new(LocalAddr => $addr, LocalPort => 0, Listen => 8, ReuseAddr => 1, Proto => "tcp")
			or die "listen $addr: $!\n";
		open(my $fh, ">", "$portfile.tmp") or die "$!\n"; print $fh $srv->sockport, "\n"; close $fh;
		rename("$portfile.tmp", $portfile) or die "$!\n";
		local $SIG{ALRM} = sub { exit 0 }; alarm 120;
		while (my $c = $srv->accept) { print $c "$token\n"; close $c; }
	' "$1" "$token" "$workdir/$2.port" &
	pids+=("$!")
	for _ in $(seq 50); do [ -s "$workdir/$2.port" ] && return 0; sleep 0.1; done
	echo "row6-probe: the $1 listener did not start" >&2
	exit 2
}

start_listener 127.0.0.1 loopback
start_listener 0.0.0.0 wildcard
loopback_port=$(cat "$workdir/loopback.port")
wildcard_port=$(cat "$workdir/wildcard.port")

echo "engine: $(docker info --format '{{.OperatingSystem}} / server {{.ServerVersion}}' 2>&1)"
docker run --rm --add-host host.docker.internal:host-gateway \
	-e TOKEN="$token" -e LOOPBACK_PORT="$loopback_port" -e WILDCARD_PORT="$wildcard_port" \
	"$image" bash -c '
		probe() {
			local line=""
			if { exec 3<>"/dev/tcp/host.docker.internal/$2"; } 2>/dev/null; then
				read -r -t 5 line <&3 || true
				exec 3<&-
			fi
			if [ "$line" = "$TOKEN" ]; then echo "$1: reachable"; else echo "$1: unreachable"; fi
		}
		probe "loopback (row 6 plan)" "$LOOPBACK_PORT"
		probe "all-interfaces (opt-in control)" "$WILDCARD_PORT"
	'
