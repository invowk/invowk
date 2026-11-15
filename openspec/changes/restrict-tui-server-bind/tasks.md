# Tasks

## 1. Endpoint authentication (D5)

- [ ] 1.1 Add one auth middleware in `internal/tuiserver/server.go` that runs before the method check on `/tui` and `/health`. It compares with `crypto/subtle.ConstantTimeCompare` and answers `401 unauthorized` on failure. Verify with new tests: `GET /health` without a token gives `401` with a body that has no "ok"/"invowk"; with the token, `200`; `GET /tui` without a token, `401`.
- [ ] 1.2 Update `TestServerMethodNotAllowed` (`server_test.go:322-350`) to send the token and still expect `405`. Calibrate by moving the auth check back behind the method check: the `GET /tui` no-token test must then fail. Verify that the constant-time path is the only comparison left (`grep -n 'expectedAuth\|!= "Bearer' internal/tuiserver/` returns nothing).
- [ ] 1.3 Send `Authorization: Bearer <token>` from `tuiclient.IsAvailableContext` (`internal/tuiclient/client.go:155-173`). Verify with a client test against a real server: missing token means unavailable, correct token means available.
- [ ] 1.4 Check: `go test ./internal/tuiserver/ ./internal/tuiclient/`

## 2. Multi-address bind (D4)

- [ ] 2.1 Let `tuiserver.New` take the bind addresses (loopback default). Listen on `127.0.0.1:0` first, then on every other address at the same port, with an `EADDRINUSE` retry bounded at 5. Serve every listener from one `http.Server`, track each serve goroutine in `serverbase`, and close all listeners on `Stop` and on start failure. Update the type comment (`server.go:34-35`). Verify: a test asserts that no listener address is unspecified and that a non-loopback host IP refuses the port under the default plan.
- [ ] 2.2 Add a test that binds a second address (a second loopback IP such as `127.0.0.2`, skipped where the OS cannot bind it). It asserts the same port on both, that both are served, a clean `Stop`, and no goroutine leak (`TestServerNoGoroutineLeaksAfterStop` pattern).
- [ ] 2.3 Check: `go test -race -run 'TestServer' ./internal/tuiserver/`

## 3. Opt-in config (D7)

- [ ] 3.1 Add `tui_server_bind: *"restricted" | "all-interfaces"` to `#ContainerConfig` (`internal/config/config_schema.cue:66-72`), with a typed Go value type that has `Validate()`, the JSON tag, and defaults in `internal/config`. Verify: a config with `"all-interfaces"` loads, and an unknown value is rejected with a CUE error.
- [ ] 3.2 Document the key in `website/docs/reference/config-schema.mdx` and its pt-BR translation, including the exposure warning. Verify with `cd website && npm run build`.
- [ ] 3.3 Check: `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`

## 4. Engine facts, plan selection and wiring (D1, D2, D3, D6)

- [ ] 4.1 Add an engine network-facts query in `internal/container`. Docker: `info` OperatingSystem and SecurityOptions, context endpoint scheme, bridge gateway. Podman: `info` Rootless, RootlessNetworkCmd and ServiceIsRemote, and the rootful `podman` network gateway. It runs through the sandbox-aware engine, with results cached per process. Verify with unit tests on a fake command runner, plus an engine-gated test on the local engine.
- [ ] 4.2 Implement the pure plan selection for the D2 table: rows in order, first match wins. Include the local-interface check for the gateway, the Flatpak no-host-network case (`/.flatpak-info` without `network` in `[Context] shared`) and the pasta-binary check (`.Host.Pasta.Executable`). Verify with a table-driven test that covers every row and the row precedence (Docker Desktop before the gateway check, `ServiceIsRemote` before any Podman row), plus the opt-in override, which never applies to host runtimes.
- [ ] 4.3 Verify rootful Podman with `sudo podman` (available on Ubuntu CI runners and on any host with sudo). Run the design.md reachability probe against the gateway that `podman network inspect podman` reports. Promote row 7 to loopback + gateway only if the probe passes. If no rootful host is available, row 7 ships unroutable. Verify by recording the outcome in design.md's probe table.
- [ ] 4.4 Verify Docker Desktop and Podman machine on a macOS or Windows host. Confirm that a server bound to `127.0.0.1` is reachable through the engine alias. If it is not, move row 3 to unroutable. If it is, Podman machine may get a loopback row ahead of row 4. If no such host is available, rows 3 and 4 ship as written: loopback-only, so it cannot widen exposure. Verify by recording the outcome in the probe table.
- [ ] 4.5 Add a validated `NetworkMode` to `container.RunOptions`, emitted as `--network <mode>` on `run` only. Keep the Podman `--userns=keep-id` image-position scan (`internal/container/podman.go:200-230`) correct for the new flag. Verify: an engine argument test with `--network pasta:-T,<port>` and the image at the right position.
- [ ] 4.6 Wire the plan into `internal/app/commandadapters/interactive.go`. Plan before `New`/`Start`. Fail an unroutable plan without the opt-in with an error that names `container.tui_server_bind` and the reason. Print the warning under the opt-in. Set `ctx.TUI.ServerURL` from the plan's dial host, and pass the network mode into container preparation (`internal/runtime/container_exec.go:189-194`, `container_prepare.go:68-79`). Verify with adapter tests: an unroutable plan errors before `PrepareInteractive` is called, and the URL carries the IP literal.
- [ ] 4.7 Check: `go test ./internal/container/ ./internal/runtime/ ./internal/app/commandadapters/`

## 5. Container integration tests (spec scenarios)

- [ ] 5.1 Add an engine-gated Go integration test (`ContainerTestContext`, suite lock). It starts the TUI server with the plan for the local engine and runs `debian:stable-slim`, which sends an authenticated `GET /health` to `INVOWK_TUI_ADDR` over bash `/dev/tcp`. Expect `200`, and `401` without the token. Verify that it passes on rootful Docker and on rootless Podman (pasta). In the CI Podman job (`.github/workflows/ci.yml:161-166`, Ubuntu Podman 4.x), either install a pinned `passt` so the job covers row 9, or assert the row 10 unroutable error there. Record which one in design.md. Where a Flatpak environment is available, run the same test there and replace "inferred" in design.md with the result.
- [ ] 5.2 In the same test, assert that the port is refused on a non-loopback, non-gateway host address. Under pasta, also assert that a second host loopback port is unreachable from the container. Calibrate by temporarily binding `0.0.0.0` and confirming that the test fails.
- [ ] 5.3 Add a testscript (`tests/cli/testdata/container_tui_unroutable.txtar`, a `container_*` test that needs no mirror) for the fail-fast path. Use a forced remote endpoint such as `DOCKER_HOST=tcp://127.0.0.1:1` with `-i`, and expect a non-zero exit, an stderr that names `container.tui_server_bind`, and no container created.
- [ ] 5.4 Check: `make test-integration`

## 6. Docs and tracking

- [ ] 6.1 Update `website/docs/advanced/interactive-mode.mdx` (Container Runtime Support, `:111-126`) and its pt-BR translation with the bind rules, the unroutable cases, the opt-in and the residual same-bridge exposure. Update the README where it describes interactive mode. Verify with `cd website && npm run build`.
- [ ] 6.2 Reword the e13 comments in `sonar-project.properties:65-67` and `.sonarcloud.properties:67-69`. Drop "Open: it binds 0.0.0.0" and state the restricted bind. Keep the IDs gapless. Verify with `make sonar-local`.
- [ ] 6.3 Mark tracker N5 in `.claude/issues/cst-adoption-gaps.md` as done, with the merged PR. Verify with `make check-agent-docs`.
- [ ] 6.4 Check: `cd website && npm run build`

## 7. Completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 `/simplify` on the diff, then `make tidy` and `make license-check`
- [ ] 7.2 `make lint`, with zero issues
- [ ] 7.3 `make test`, the full suite with container tests on an engine-equipped host
- [ ] 7.4 `make check-baseline`, triaging every new goplint finding (no baselining of inconclusives)
- [ ] 7.5 `make check-file-length` and `make sonar-local`
- [ ] 7.6 `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`, `make check-windows-build` and `make check-agent-docs`
- [ ] 7.7 Re-check `docs/architecture/` for the interactive flow, and `/learn`
- [ ] 7.8 `openspec validate restrict-tui-server-bind --strict`
