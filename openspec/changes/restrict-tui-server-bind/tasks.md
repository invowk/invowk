# Tasks

## 1. Request authentication (D5)

- [x] 1.1 Wrap the whole mux in `internal/tuiserver/server.go` (`:87-89`) in one auth middleware. It runs before routing and before the method check, compares with `crypto/subtle.ConstantTimeCompare`, and answers `401` with the body `unauthorized` on failure. Verify with new tests: *(2026-11-15: `requireToken` middleware; `TestServerAuthCoversEveryPath`.)*
  - `GET /health` without a token gives `401` and the body `unauthorized`; with the token, `200`;
  - `GET /tui` without a token gives `401`;
  - `GET /` and `GET /nope` without a token give `401`; `GET /nope` with the token gives `404`.
- [x] 1.2 Update `TestServerMethodNotAllowed` (`server_test.go:322-350`) to send the token and still expect `405`. Calibrate by moving the auth check back behind the method check: the `GET /tui` no-token test must then fail. Confirm that the constant-time path is the only comparison left: `grep -n 'expectedAuth\|!= "Bearer' internal/tuiserver/` returns nothing. *(2026-11-15: calibrated by serving the bare mux: the `/tui` no-token case failed; the grep returns nothing.)*
- [x] 1.3 Send `Authorization: Bearer <token>` from `tuiclient.IsAvailableContext` (`internal/tuiclient/client.go:155-173`). Extend `TestClientIsAvailable` (`client_test.go:532-556`) against a real server: a wrong token means unavailable, the right token means available. *(2026-11-15: `TestClientIsAvailable` covers the wrong token.)*
- [x] 1.4 Give the client transport a 5-second dial timeout and keep the 10-minute overall timeout (D9). Map dial failures to `ErrTUIServerUnreachable`, whose message names the address and `container.tui_server_bind`. Verify with tests: a refused port (a closed listener) and a blackholed address (`192.0.2.1`, TEST-NET-1, with the dial timeout shortened through an unexported test seam) both return `ErrTUIServerUnreachable` within the timeout; a slow handler that answers after the dial timeout still succeeds. *(2026-11-15: `UnreachableError`; `TestClientUnreachableServerFailsFast`, `TestClientDialTimeoutDoesNotBoundInteraction`.)*
- [x] 1.5 Check: `go test ./internal/tuiserver/ ./internal/tuiclient/` *(2026-11-15: passed with `-race`.)*

## 2. Multi-address bind (D4)

- [x] 2.1 Let `tuiserver.New` take a bind plan: a list of addresses (`[]netip.Addr`, default `127.0.0.1` only, unspecified addresses rejected), or the explicit wildcard kind, which is one `0.0.0.0:0` listener and nothing else (D4). For an address list, `Start` does the following: *(2026-11-15: `BindPlan`, `NewWithBindPlan`, `ListenAddrs`; `server_bind_test.go`.)*
  - listen on `127.0.0.1:0`, then on every other address at the same port;
  - on any later listen error, close all listeners and retry, at most 5 attempts;
  - start one tracked `Serve` goroutine per listener on one `http.Server`, then call `TransitionToRunning` once;
  - on a `Serve` error from any listener, fail the server and shut down the rest.

  `Stop` closes every listener. Update the type comment (`server.go:34-35`). Verify with tests:
  - under the default plan, every listener address is `127.0.0.1`;
  - an address list containing `0.0.0.0` is rejected by `New`, and the wildcard kind yields exactly one listener on `0.0.0.0`;
  - a dial to each non-loopback local address at the port is refused. The test skips with an explicit message when the host has none.
- [x] 2.2 Add a Linux-only test that binds `127.0.0.1` and `127.0.0.2`. It asserts the same port on both, that both serve authenticated `/health`, a clean `Stop` and no goroutine leak (the `TestServerNoGoroutineLeaksAfterStop` pattern, `server_test.go:472`). Add a test where the second address cannot be bound (for example `192.0.2.1`, TEST-NET-1): `Start` fails after 5 attempts, the state is Failed, and the port of the first listener is closed again. *(2026-11-15: `server_bind_linux_test.go`; `TestServerUnbindableSecondAddressFails`.)*
- [x] 2.3 Check: `go test -race -run 'TestServer' ./internal/tuiserver/` *(2026-11-15: passed.)*

## 3. Opt-in config (D7, D10)

- [x] 3.1 Add the config key and its Go support: *(2026-11-15: `internal/config/tui_server_bind.go`; `tui_server_bind_test.go` (INVOWK_CONFIG source, round trip, unknown value); `pkg/invowkfile/tui_server_bind_boundary_test.go` (invowkfile and invowkmod manifest, each with a control fixture); `config show` gains a `container` section; goldens updated.)*
  - `tui_server_bind: *"restricted" | "all-interfaces"` in `#ContainerConfig` (`internal/config/config_schema.cue:66-71`);
  - a typed Go value type with `Validate()`, the JSON and mapstructure tags beside `AutoProvision` (`internal/config/types.go:343`), and the default in `DefaultConfig`;
  - emission in `writeConfigCUEContainer` (`internal/config/config.go:592-600`);
  - the `config_show`/`config_dump` testscript goldens (`tests/cli/testdata/config_show.txtar`, `config_dump.txtar`).

  Verify:
  - a config with `"all-interfaces"` loads and round-trips through `GenerateCUE`;
  - an unknown value is rejected with a CUE error;
  - trust boundary (D10): a config file selected through `INVOWK_CONFIG` with `"all-interfaces"` yields that value in the loaded config, and an invowkfile whose container runtime block sets `tui_server_bind` fails `invowkfile` parsing (a Go test in `pkg/invowkfile`).
- [x] 3.2 Document the key in `website/docs/reference/config-schema.mdx` and its pt-BR translation, including the exposure warning, that it is user configuration only, that it does not apply in Flatpak without network, and that for remote engines it helps only when the alias leads back to this machine. Verify with `cd website && npm run build`. *(2026-11-15: en + pt-BR plus the `container-tui-server-bind` snippet; website build passed.)*
- [x] 3.3 Check: `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/` *(2026-11-15: passed.)*

## 4. Engine facts, plan selection and wiring (D1, D2, D3, D6, D8, D11)

- [x] 4.1 Add an engine network-facts query in `internal/container`: *(2026-11-15: `internal/container/network_facts.go`, `RunQuery` on both engines; `network_facts_test.go`, local Docker run reported `BridgeGateway:172.17.0.1`.)*
  - Docker: `info` OperatingSystem and SecurityOptions, the endpoint (`DOCKER_HOST`, otherwise `docker context inspect`), and the bridge gateway;
  - Podman: `info` Rootless, RootlessNetworkCmd, ServiceIsRemote and Pasta.Executable;
  - Flatpak: whether `/.flatpak-info` `[Context] shared` contains `network`.

  The query runs through the sandbox-aware engine, only when the planner asks for it, and its results are cached per container runtime instance. Verify with unit tests on a fake command runner, including a failing `info` call and a `podman-remote` result, plus an engine-gated test on the local engine.
- [x] 4.2 Implement the pure plan selection for the D2 table in `internal/containerplan`, evaluating the rows in order. Verify with a table-driven test that covers: *(2026-11-15: `internal/containerplan/tuiexposure.go`; `tuiexposure_test.go`.)*
  - every row, plus row precedence: Flatpak before remote, remote before any Docker or Podman row, Docker Desktop before the gateway check;
  - the gateway-not-local case (row 13);
  - the opt-in: it turns restorable rows and row 6 into the wildcard plan with the engine alias, and never changes rows 1, 2, 8 or 11;
  - the no-server plans: rows 2-5, 7, 9, 10, 12 and 13 yield kind `none`; restorable warnings name `container.tui_server_bind: "all-interfaces"`, row 3 adds the alias caveat, row 12 also names pasta, and the row 2 warning does not name `all-interfaces`.
- [x] 4.3 Add a validated `NetworkMode` to `container.RunOptions`, emitted as the single argument `--network=<mode>` on `run` only. Verify with an engine argument test: for `--network=pasta:-T,<port>` plus `-p`, `--add-host` and `-e`, the Podman `--userns=keep-id` transformer (`internal/container/podman.go:200-230`) places its flag before the image, and `exec`/`create` arguments never contain `--network`. *(2026-11-15: `internal/container/network_mode.go`; `TestPodmanRunArgsNetworkModeBeforeImage`.)*
- [x] 4.4 Wire the plan into the code (decision 21): *(2026-11-15: planner on `ContainerRuntime` (`container_tui_exposure.go`), consumer-side `tuiExposurePlanner` in the adapter; `interactive_exposure_test.go`, `interactive_exposure_linux_test.go`, `TestContainerRuntimePlanTUIExposure`, `TestContainerPrepareCommandTUINetworkMode`.)*
  - the container runtime implements `PlanTUIExposure` (the adapter-side `tuiExposurePlanner` port) from its engine and its `*config.Config`; native and virtual runtimes get the loopback plan;
  - the interactive adapter plans before `New`/`Start`; for kind `none` it writes the warning once to `ctx.IO.Stderr` before the alternate screen, starts no server, leaves `ctx.TUI` empty, skips the bridge and `Stop`, and runs the command; for kind `wide` it also writes the exposure warning;
  - set `ctx.TUI.ServerURL` from the plan's dial host;
  - pass the network mode, with the port filled in, into container preparation (`internal/runtime/container_exec.go:189-194`, `container_prepare.go:69-80`).

  Verify with adapter tests using a fake planner and a spy runtime:
  - a `none` plan never calls `tuiserver.Start`, still calls `PrepareInteractive` with an empty `ctx.TUI`, propagates the command's exit code and writes the warning to stderr exactly once;
  - the URL carries the IP literal for row 8 and `127.0.0.1` for row 11;
  - under the opt-in, a restorable plan and a row 6 plan start a server whose only listener is `0.0.0.0`, set the engine alias in the URL, and write the exposure warning to stderr; a row 8 plan under the opt-in writes no warning.
- [x] 4.5 Non-interactive runs (D11): a test runs a non-interactive container request through `commandsvc` with a spy facts provider and a spy interactive executor, and asserts zero facts queries, zero executor calls and no warning on stderr. A second test asserts that `NewInteractiveExecutor` and `NewContainerRuntime` query no facts. *(2026-11-15: new `TestExecuteWithRequestedMode` case with an interactive-capable runtime; `TestContainerRuntimeConstructionQueriesNoFacts`.)*
- [x] 4.6 Check: `go test ./internal/container/ ./internal/containerplan/ ./internal/runtime/ ./internal/app/commandadapters/ ./internal/app/commandsvc/` *(2026-11-15: passed, plus `./internal/tuiserver/ ./internal/tuiclient/ ./internal/config/ ./cmd/...`.)*

## 5. Container integration tests (spec scenarios)

- [x] 5.1 Add an engine-gated Go integration test (`testutil.ContainerTestContext`, suite lock) in `internal/app/commandadapters`. It runs the production path: the real facts query, the planner, `tuiserver.Start` and `ContainerRuntime.PrepareInteractive`. It then runs the prepared command under a PTY (`github.com/creack/pty`, already in `go.mod`). The `debian:stable-slim` script sends `GET /health` to `INVOWK_TUI_ADDR` over bash `/dev/tcp`, once with `INVOWK_TUI_TOKEN` and once without. The test expects `200` and then `401`. *(2026-11-15: `interactive_exposure_integration_test.go` (Linux); the PTY stands in for the Bubble Tea session through the executor's `runSession` seam. Local: row 8 on rootful Docker, row 11 on host rootless Podman 6.1.3 with pasta. `ci.yml`: Docker `"8"`; Podman set to `"11"` after the first run (43677325230) printed `true|pasta|false|/usr/bin/pasta` and selected row 11; recorded in design.md, and the spec/README say CI verifies the pasta plan.)*
  - The test reads the expected D2 row from `INVOWK_TEST_TUI_PLAN_ROW` and fails if the selected row differs. If the variable is unset, it fails in CI and skips with a message locally.
  - In `.github/workflows/ci.yml`, the Podman job prints `podman info --format '{{.Host.RootlessNetworkCmd}}|{{.Host.Pasta.Executable}}'` before the tests. Set the variable as a literal: `"8"` for the Docker job; for the Podman job, `"11"` if that first run shows a pasta executable, else `"12"`. Under row 12 the test asserts the no-server run instead of reachability: no `INVOWK_TUI_*` in the container and the warning once. Record the observed backend in design.md's probe table. If the runner has no pasta, state in the spec requirement and the README that the pasta plan is verified locally only.
  - Locally, run it on rootful Docker (row 8) and, where available, on rootless Podman with pasta (row 11).
- [x] 5.2 In the same test, assert from the host side that the port refuses connections on every local address outside the plan. Dial each address from `net.InterfaceAddrs()` that is neither loopback nor the gateway, and skip loudly if none exists. Under pasta, also assert from the container that a second host loopback listener is unreachable. Calibrate both: *(2026-11-15: both calibrations failed as required; the pasta one forwarded the second port too, since `--map-host-loopback` maps host loopback to another address that a 127.0.0.1 probe would not exercise.)*
  - temporarily bind `0.0.0.0`: the host-side check must fail;
  - temporarily use `pasta:--map-host-loopback`: the second-port check must fail.
- [x] 5.3 Add an engine-gated Go integration test for the no-server path on the production adapter (`InteractiveExecutor.Execute` under a PTY), with a planner that returns a no-route plan on the local engine. The `debian:stable-slim` script prints whether `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` are set and runs a delegating `invowk tui spin --title t -- true` in direct rendering, then exits with a chosen code. Expect: both variables unset, the spin call succeeds, the chosen exit code propagates, and stderr holds the warning exactly once. Under rootless Podman, also run it with the real planner against a persistent target (row 10). *(2026-11-15: `TestInteractiveNoServerRunRendersDirectly` (forced row 7, `invowk tui spin` direct, exit 7, one warning) and `TestInteractivePersistentRootlessPodmanRunsWithoutServer` (real planner, row 10; passed on host Podman).)*
- [x] 5.4 Add a test for the unreachable server from inside a container: run `invowk tui confirm` in a `debian:stable-slim` container with `INVOWK_TUI_ADDR` pointing at a closed port on the bridge gateway. Expect a non-zero exit within the dial timeout and stderr naming `container.tui_server_bind`. *(2026-11-15: `TestTUIClientUnreachableFromContainer`: refused 127.0.0.1:1 in 0 s, dropped 192.0.2.1 in 5 s, both naming the opt-in.)*
- [x] 5.5 Check: `make test-integration` *(2026-11-15: passed on rootful Docker with `INVOWK_TEST_TUI_PLAN_ROW=8`.)*

## 6. Docs and tracking

- [x] 6.1 Update `website/docs/advanced/interactive-mode.mdx` (Container Runtime Support, `:111-126`) and its pt-BR translation. Cover the per-mode bind rules, the no-server runs with direct rendering and their warning, the Flatpak-without-network case, the opt-in (user config only; also the Docker Desktop remedy), the unreachable-server error, the residual same-bridge and on-link exposure, firewalls, and external persistent containers. Update the README where it describes interactive mode. Re-check `website/docs/runtime-modes/container.mdx:133` (the alias example stays correct for SSH). Verify with `cd website && npm run build`. *(2026-11-15: "Network exposure of the TUI server" in en + pt-BR; README interactive-mode paragraph and config comment; `container.mdx:133` unchanged (SSH example). Website build passed.)*
- [x] 6.2 Reword the e13 comments in `sonar-project.properties:65-67` and `.sonarcloud.properties:67-69`. Drop "Open: it binds 0.0.0.0" and state the restricted bind. Keep the IDs gapless. Verify with `make sonar-local`. *(2026-11-15: `make sonar-local` passed in the 069bba39 pre-commit hooks.)*
- [x] 6.3 Tracking: mark tracker N5 in `.claude/issues/cst-adoption-gaps.md` as implemented in this change (done on merge); add N7 for the SSH host-callback bind (decision 20); and change `tasks/next/formal-phase3-closeout.md:11` from "false positive" to "Accepted; real gap N5/S3". Verify with `make check-agent-docs`. *(2026-11-15: N5 marked implemented (done on merge), N7 added, closeout line 11 reworded; `make check-agent-docs` passed.)*
- [x] 6.4 Check: `cd website && npm run build` *(2026-11-15: passed, no broken links or anchors.)*

## 7. Completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 `/simplify` on the diff, then `make tidy` and `make license-check`
- [ ] 7.2 `make lint`, with zero issues
- [ ] 7.3 `make test`, the full suite with container tests on an engine-equipped host
- [ ] 7.4 `make check-baseline`, triaging every new goplint finding (inconclusives are never baselined)
- [ ] 7.5 `make check-file-length` and `make sonar-local`
- [ ] 7.6 `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`, `make check-windows-build` and `make check-agent-docs`
- [ ] 7.7 Re-check `docs/architecture/` for the interactive flow (`c4-component-runtime.md:41`), and run `/learn`
- [ ] 7.8 `openspec validate restrict-tui-server-bind --strict`
