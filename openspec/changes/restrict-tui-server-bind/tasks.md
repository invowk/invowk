# Tasks

## 1. Request authentication (D5)

- [ ] 1.1 Wrap the whole mux in `internal/tuiserver/server.go` (`:87-89`) in one auth middleware. It runs before routing and before the method check, compares with `crypto/subtle.ConstantTimeCompare`, and answers `401` with the body `unauthorized` on failure. Verify with new tests:
  - `GET /health` without a token gives `401` and the body `unauthorized`; with the token, `200`;
  - `GET /tui` without a token gives `401`;
  - `GET /` and `GET /nope` without a token give `401`; `GET /nope` with the token gives `404`.
- [ ] 1.2 Update `TestServerMethodNotAllowed` (`server_test.go:322-350`) to send the token and still expect `405`. Calibrate by moving the auth check back behind the method check: the `GET /tui` no-token test must then fail. Confirm that the constant-time path is the only comparison left: `grep -n 'expectedAuth\|!= "Bearer' internal/tuiserver/` returns nothing.
- [ ] 1.3 Send `Authorization: Bearer <token>` from `tuiclient.IsAvailableContext` (`internal/tuiclient/client.go:155-173`). Extend `TestClientIsAvailable` (`client_test.go:532-556`) against a real server: a wrong token means unavailable, the right token means available.
- [ ] 1.4 Check: `go test ./internal/tuiserver/ ./internal/tuiclient/`

## 2. Multi-address bind (D4)

- [ ] 2.1 Let `tuiserver.New` take the bind addresses (`[]netip.Addr`, default `127.0.0.1` only; reject unspecified addresses). `Start` does the following:
  - listen on `127.0.0.1:0`, then on every other address at the same port;
  - on any later listen error, close all listeners and retry, at most 5 attempts;
  - start one tracked `Serve` goroutine per listener on one `http.Server`, then call `TransitionToRunning` once;
  - on a `Serve` error from any listener, fail the server and shut down the rest.

  `Stop` closes every listener. Update the type comment (`server.go:34-35`). Verify with tests:
  - under the default plan, every listener address is `127.0.0.1`;
  - a dial to each non-loopback local address at the port is refused. The test skips with an explicit message when the host has none.
- [ ] 2.2 Add a Linux-only test that binds `127.0.0.1` and `127.0.0.2`. It asserts the same port on both, that both serve authenticated `/health`, a clean `Stop` and no goroutine leak (the `TestServerNoGoroutineLeaksAfterStop` pattern, `server_test.go:472`). Add a test where the second address cannot be bound (for example `192.0.2.1`, TEST-NET-1): `Start` fails after 5 attempts, the state is Failed, and the port of the first listener is closed again.
- [ ] 2.3 Check: `go test -race -run 'TestServer' ./internal/tuiserver/`

## 3. Opt-in config (D7)

- [ ] 3.1 Add the config key and its Go support:
  - `tui_server_bind: *"restricted" | "all-interfaces"` in `#ContainerConfig` (`internal/config/config_schema.cue:66-71`);
  - a typed Go value type with `Validate()`, the JSON and mapstructure tags beside `AutoProvision` (`internal/config/types.go:343`), and the default in `DefaultConfig`;
  - emission in `writeConfigCUEContainer` (`internal/config/config.go:592-600`);
  - the `config_show`/`config_dump` testscript goldens (`tests/cli/testdata/config_show.txtar`, `config_dump.txtar`).

  Verify:
  - a config with `"all-interfaces"` loads and round-trips through `GenerateCUE`;
  - an unknown value is rejected with a CUE error.
- [ ] 3.2 Document the key in `website/docs/reference/config-schema.mdx` and its pt-BR translation, including the exposure warning and that it does not apply to remote engines. Verify with `cd website && npm run build`.
- [ ] 3.3 Check: `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`

## 4. Engine facts, plan selection and wiring (D1, D2, D3, D6)

- [ ] 4.1 Add an engine network-facts query in `internal/container`:
  - Docker: `info` OperatingSystem and SecurityOptions, the endpoint (`DOCKER_HOST`, otherwise `docker context inspect`), and the bridge gateway;
  - Podman: `info` Rootless, RootlessNetworkCmd, ServiceIsRemote and Pasta.Executable;
  - Flatpak: whether `/.flatpak-info` `[Context] shared` contains `network`.

  The query runs through the sandbox-aware engine, and its results are cached per process. Verify with unit tests on a fake command runner, including a failing `info` call and a `podman-remote` result, plus an engine-gated test on the local engine.
- [ ] 4.2 Implement the pure plan selection for the D2 table in `internal/containerplan`, evaluating the rows in order. Verify with a table-driven test that covers:
  - every row, plus row precedence: Flatpak before remote, remote before any Docker or Podman row, Docker Desktop before the gateway check;
  - the gateway-not-local case (row 13);
  - the opt-in, which changes only restorable rows, never routable rows, no-route rows or host runtimes;
  - the error text: restorable errors name `container.tui_server_bind`, row 12 also names pasta, and no-route errors do not name `all-interfaces`.
- [ ] 4.3 Add a validated `NetworkMode` to `container.RunOptions`, emitted as the single argument `--network=<mode>` on `run` only. Verify with an engine argument test: for `--network=pasta:-T,<port>` plus `-p`, `--add-host` and `-e`, the Podman `--userns=keep-id` transformer (`internal/container/podman.go:200-230`) places its flag before the image, and `exec`/`create` arguments never contain `--network`.
- [ ] 4.4 Wire the plan into the code, in this order:
  - inject the bind policy through `NewInteractiveExecutor` (`internal/app/commandadapters/interactive.go:26`) from the loaded config;
  - plan before `New`/`Start`; fail an unroutable plan with its error, and print the warning when the opt-in widens;
  - set `ctx.TUI.ServerURL` from the plan's dial host;
  - pass the network mode, with the port filled in, into container preparation (`internal/runtime/container_exec.go:189-194`, `container_prepare.go:69-80`).

  Verify with adapter tests using a fake planner and a spy runtime:
  - an unroutable plan errors before `tuiserver.Start` and `PrepareInteractive` are called;
  - the URL carries the IP literal for row 8 and `127.0.0.1` for row 11.
- [ ] 4.5 Check: `go test ./internal/container/ ./internal/containerplan/ ./internal/runtime/ ./internal/app/commandadapters/`

## 5. Container integration tests (spec scenarios)

- [ ] 5.1 Add an engine-gated Go integration test (`testutil.ContainerTestContext`, suite lock) in `internal/app/commandadapters`. It runs the production path: the real facts query, the planner, `tuiserver.Start` and `ContainerRuntime.PrepareInteractive`. It then runs the prepared command under a PTY (`github.com/creack/pty`, already in `go.mod`). The `debian:stable-slim` script sends `GET /health` to `INVOWK_TUI_ADDR` over bash `/dev/tcp`, once with `INVOWK_TUI_TOKEN` and once without. The test expects `200` and then `401`.
  - The test reads the expected D2 row from `INVOWK_TEST_TUI_PLAN_ROW` and fails if the selected row differs. If the variable is unset, it fails in CI and skips with a message locally.
  - The CI Docker job sets row 8, and the CI Podman job sets the row that its `podman info` reports (11 with pasta, otherwise 12, where the test asserts the error instead). Record the observed row in design.md's probe table.
  - Locally, run it on rootful Docker (row 8) and on rootless Podman with pasta (row 11).
- [ ] 5.2 In the same test, assert from the host side that the port refuses connections on every local address outside the plan. Dial each address from `net.InterfaceAddrs()` that is neither loopback nor the gateway, and skip loudly if none exists. Under pasta, also assert from the container that a second host loopback listener is unreachable. Calibrate both:
  - temporarily bind `0.0.0.0`: the host-side check must fail;
  - temporarily use `pasta:--map-host-loopback`: the second-port check must fail.
- [ ] 5.3 Add an engine-gated Go integration test for the restorable fail-fast path that runs only under rootless Podman (row 10). It builds the `invowk` binary and runs `invowk cmd <persistent-target> -i` with a fresh container name. It expects a non-zero exit, an error that names `container.tui_server_bind`, and `podman ps -a --filter name=<name>` empty afterwards. Do not use a dead `DOCKER_HOST`: it fails `Available()` (`internal/container/docker.go:53-61`) before the planner runs. Cover the no-route rows with the unit tests in 4.2 and 4.4.
- [ ] 5.4 Check: `make test-integration`

## 6. Docs and tracking

- [ ] 6.1 Update `website/docs/advanced/interactive-mode.mdx` (Container Runtime Support, `:111-126`) and its pt-BR translation. Cover the per-mode bind rules, the restorable and no-route errors, the opt-in, the residual same-bridge and on-link exposure, firewalls, and external persistent containers. Update the README where it describes interactive mode. Re-check `website/docs/runtime-modes/container.mdx:133` (the alias example stays correct for SSH). Verify with `cd website && npm run build`.
- [ ] 6.2 Reword the e13 comments in `sonar-project.properties:65-67` and `.sonarcloud.properties:67-69`. Drop "Open: it binds 0.0.0.0" and state the restricted bind. Keep the IDs gapless. Verify with `make sonar-local`.
- [ ] 6.3 Mark tracker N5 in `.claude/issues/cst-adoption-gaps.md` as done, with the merged PR. Verify with `make check-agent-docs`.
- [ ] 6.4 Check: `cd website && npm run build`

## 7. Completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 `/simplify` on the diff, then `make tidy` and `make license-check`
- [ ] 7.2 `make lint`, with zero issues
- [ ] 7.3 `make test`, the full suite with container tests on an engine-equipped host
- [ ] 7.4 `make check-baseline`, triaging every new goplint finding (inconclusives are never baselined)
- [ ] 7.5 `make check-file-length` and `make sonar-local`
- [ ] 7.6 `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`, `make check-windows-build` and `make check-agent-docs`
- [ ] 7.7 Re-check `docs/architecture/` for the interactive flow (`c4-component-runtime.md:41`), and run `/learn`
- [ ] 7.8 `openspec validate restrict-tui-server-bind --strict`
