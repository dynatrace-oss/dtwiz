# Tasks: otel-update-non-dynatrace-gateway

## 0. Collector classification

- [x] 0.1 In `pkg/installer/otel/update.go`, after `selectCollector` resolves the chosen collector, classify it as Dynatrace vs. non-Dynatrace using the existing `isDynatrace` field. Dynatrace selections use the existing `mergeDynatraceExporter` path unchanged; non-Dynatrace selections route to `UpdateNonDynatraceCollector` (`pkg/installer/otel/gateway.go`).

## 1. Config-source validation

- [x] 1.1 `validateConfigSource` (`gateway.go`) determines whether a collector's effective config resolves to a single, writable, local YAML file, returning a clear reason string when it does not (multiple `--config` flags, `env:`/`yaml:` provider, non-durable container-baked config, unwritable path). Covered by `TestValidateConfigSource` and `TestCountConfigFlags`/`TestLooksLikeInlineProvider`.
- [x] 1.2 On failure, `printConfigSourceFallback` shows a docs link and `UpdateNonDynatraceCollector` returns immediately — no backup, no gateway deployment, no config write. Covered end-to-end by `TestUpdateNonDynatraceCollector_InvalidConfigSourceMakesNoChanges`.

## 2. Backup

- [x] 2.1 On successful validation, `UpdateNonDynatraceCollector` creates a timestamped backup via the existing `backupFile` helper and prints its path before any further action.

## 3. Supervisor detection and launch-context capture

- [x] 3.1 Linux systemd-unit detection (`gateway_unix.go: detectSystemdUnit`) parses `/proc/<pid>/cgroup` for a `*.service` path. Not unit-tested (would require faked `/proc` fixtures); implemented and manually reasoned through, not exercised by an automated test.
- [x] 3.2 Container detection reused directly via `collectorInstance.containerRuntime` (existing detection) in `detectSupervisor`.
- [x] 3.3 Kubernetes pod detection (`isKubernetesPod`, cgroup contains `kubepods`) classifies as "cannot restart automatically" per design Decision 4. Not unit-tested, same caveat as 3.1.
- [x] 3.4 Full launch-context capture (`captureLaunchContext`) implemented for Linux via `/proc/<pid>/cmdline` + `environ` + `cwd`. **Scope note:** macOS and Windows always return an incomplete capture (no reliable, unprivileged way to read another process's environment block on either platform) — bare/manual collectors on those platforms always fall back to the manual-restart path in this version, per design Decision 4's "incomplete capture ⇒ cannot restart" rule. Not unit-tested.

## 4. Gateway collector deployment

- [x] 4.1 `prepareGatewayPlan` / `(*gatewayPlan).deploy` (`gateway.go`) deploy the Dynatrace Gateway Collector to its own directory (`~/opentelemetry-gateway/`), reusing `downloadOtelCollector`, `generateOtelConfig` (host monitoring is already gated behind `featureflags.Experimental` inside `generateOtelConfig`, which this flow's own `--experimental` gate satisfies), and `startOtelCollector`.
- [x] 4.2 The gateway's receiver port is resolved by re-using `generateOtelConfig`'s existing free-port probing (starting at 4317, same as the app-monitoring collector) rather than a hardcoded port — `parseOtlpGRPCPort` reads back whatever port was actually assigned. Covered by `TestParseOtlpGRPCPort`.
- [x] 4.3 `waitForPortOpen` blocks until the gateway's receiver port accepts connections before the foreign config is patched; a deploy or readiness failure returns an error before any write to the foreign collector's config (verified by code path: the foreign-config write occurs strictly after the gateway-ready block in `UpdateNonDynatraceCollector`).

## 5. Foreign-config patch

- [x] 5.1 `buildGatewayExporterDef` builds the `otlp/dt-gateway` exporter definition (no auth, `tls.insecure: true`, `sending_queue.block_on_overflow: false`) as a `map[string]interface{}`, mirroring `mergeDynatraceExporter`'s technique. Covered by `TestBuildGatewayExporterDef` (including an explicit secret-hygiene assertion that no `headers`/auth key is present).
- [x] 5.2 `mergeGatewayExporter` appends the new exporter to every existing pipeline's `exporters:` list without touching any existing receiver, processor, or exporter. Covered by `TestMergeGatewayExporter_AdditiveOnly` and `TestMergeGatewayExporter_Idempotent`.
- [ ] 5.3 **Not implemented as a separate runtime assertion.** Additivity is guaranteed by construction (the merge function only ever adds a map key and appends to a list; it has no code path that deletes or overwrites existing keys) and is verified by the unit tests in 5.1/5.2, rather than by a redundant runtime diff-check before every write. Flagged here rather than silently checked off, since the task as originally written asked for an explicit validation step and this implementation takes a "correct by construction" approach instead.

## 6. Preview and confirmation

- [x] 6.1 `UpdateNonDynatraceCollector` prints the gateway collector info, the additive diff (via the existing `showConfigDiff`, or a "no change needed" note when idempotent), and the detected restart plan (or lack thereof).
- [x] 6.2 Single `Apply? [Y/n]` confirmation via the existing `installer.ShouldProceed`, which also handles `--dry-run`.

## 7. Restart handling

- [x] 7.1 Systemd path (`restartViaSystemctl`), container path (existing `restartContainer`), and bare/manual path (`relaunchWithContext`, using the captured launch context, never a synthesized command line) are all implemented and dispatched from `restartForeignCollector`.
- [x] 7.2 Kubernetes pod, ambiguous supervisor, or incomplete launch-context capture all resolve to `restartUnavailable`, routing to `printManualRestartInstructions` instead of an automatic restart attempt.
- [x] 7.3 Each restart mechanism verifies its own success using its native signal, rather than polling a network port (an arbitrary foreign collector's config may have no listening receiver at all — e.g. a scrape-only setup — making port-polling an unreliable universal check): `restartViaSystemctl` polls `systemctl is-active` until the unit reports active or `systemdActiveTimeout` (15s) elapses; `restartContainerAndVerify` polls the container's `.State.Running` after `restartContainer`; `relaunchWithContext` mirrors `startOtelCollector`'s grace-period crash check (3s) for the bare-process case. All three report a genuine restart failure (not just a non-zero exit code from the initial restart command) into the existing error path in `UpdateNonDynatraceCollector`, which restores the pre-patch config backup and falls back to manual instructions.

## 8. Post-restart outcome

- [x] 8.1 Automatic restart success calls `installer.WatchIngestWithStatus`.
- [x] 8.2 Automatic restart failure, or restart unavailable, both call `printManualRestartInstructions` (config path + backup path) and show links to the Dynatrace Services and Distributed Tracing apps instead of `WatchIngest`.

## 9. Tests

- [x] 9.1 Unit tests for config-source validation (`TestValidateConfigSource`, `TestCountConfigFlags`, `TestLooksLikeInlineProvider`) covering: single writable file passes; multi `--config`, env/yaml provider, non-durable container config, unwritable path, missing path, and directory path all fail.
- [ ] 9.2 Unit tests for supervisor classification (systemd unit, docker container, bare process with/without full launch-context capture, k8s pod) — **not implemented.** The container and "no supervisor detected" cases are implicitly exercised by other flows' existing container tests, but no dedicated test fakes `/proc/<pid>/cgroup` contents to exercise `detectSystemdUnit`/`isKubernetesPod`/`captureLaunchContext` directly.
- [x] 9.3 `TestMergeGatewayExporter_AdditiveOnly` and `TestPatchForeignConfigForForwarding` assert the foreign-config patch adds only the `otlp/dt-gateway` exporter and its pipeline references, with existing receivers/exporters/pipelines unchanged, across a config with multiple existing pipelines and exporters.
- [x] 9.4 `TestUpdateNonDynatraceCollector_InvalidConfigSourceMakesNoChanges` asserts the config-source-invalid path creates no gateway install directory and no other files, and returns a nil error (clean exit).
- [ ] 9.5 Test asserting a gateway-deploy failure aborts before any foreign-config write — **not implemented** (would require mocking `downloadOtelCollector`'s network call; not attempted in this pass).
- [ ] 9.6 Tests explicitly asserting this flow is inert unless `--experimental`/`DTWIZ_EXPERIMENTAL=true` — **not implemented as a dedicated test.** The flow is only reachable through the pre-existing `update otel` command, which is already gated at the command level (`cmd/update.go`), so no new ungated entry point was introduced; this was reasoned through rather than covered by a new automated test.

## 10. Docs

- [x] 10.1 `CHANGELOG.md` `[Unreleased]` updated noting the new non-Dynatrace `update otel` gateway flow, gated behind `--experimental`.
- [x] 10.2 `make test` and `make lint` run clean, aside from one pre-existing, unrelated failure (`TestBuildInstrumentedCmd_JavaPrefixCommand`, an environment-specific Java path assertion, confirmed via `git stash` to fail identically without any of this change's code present).
- [ ] 10.3 Manual verification against a real non-Dynatrace collector (e.g. `otelcol-contrib` under systemd, and one under docker-compose) — **not performed in this session** (no such environment available); remains as a manual QA step before this change is considered fully verified.
