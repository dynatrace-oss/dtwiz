## Context

The `ist` event was added by the `instrument-install-selfmonitoring` change (shipped in v1.10.0). It carries:

- `install.duration_s`, computed from `installer.ExecutionStart`. Every `confirmProceed` call overwrites that timestamp, and the cmd layer reads it after the installer returns.
- `install.host_monitoring_enabled` and `install.otel_pipelines`, both looked up from a fixed per-method map (`installMethodFeatures`).

That design has these gaps:

| Gap                                                                         | Cause                                                                            |
| --------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Host monitoring reported as enabled when activation failed                  | `activateHostMonitoringExtension` returns nothing; the value is fixed per method |
| AWS, Azure and GCP duration includes the watch session                      | these installers run the watch themselves and return afterwards                  |
| Python duration loses work done before the "recreate virtualenv?" prompt    | that prompt, inside `Execute()`, overwrites `ExecutionStart`                     |
| No duration for `oneagent` (setup), `docker`, `aws-lambda` without a prompt | nothing starts the clock when there is no confirmation                           |
| Setup's `ist` has no duration or features                                   | `fireSetupInstallEvent` never reads them                                         |
| Neither value survives a rejected event body                                | both are body-only                                                               |

The existing pattern for passing per-run state between installers and the cmd layer is package-level variables in `pkg/installer` (`AutoConfirm`, `ExecutionStart`, `OnWatchSnapshot`, `OnWatchComplete`). Only one install runs per process.

The User-Agent is captured by HAProxy up to 64 characters. A worst-case `ist` header today is `dtwiz/1.10.0;c=set;st=ist;s=awsl;er=ifl` (39 chars).

## Goals / Non-Goals

**Goals:**

- Report the real three-state outcome of each feature, recorded where the work happens.
- Report install work time that excludes think time at any prompt and the post-install watch.
- Carry both in the User-Agent and in readable body properties.
- Same `ist` content for `dtwiz install <method>` and for `dtwiz setup`.

**Non-Goals:**

- Direct `dtwiz update otel|azure|gcp` commands. They keep their current events; setup's update choices are in scope.
- Wiring RUM, Synthetic or RDS. Their positions are reserved and always `-`.
- Enabling host monitoring in the Kubernetes DynaKube.
- Per-phase durations (e.g. OneAgent download vs. install).
- Changing which steps fire, or when. Setup still sends no `ist` on cancellation or when the OTel config is up to date.

## Decisions

### Positional three-state string for `f=`, not a bitmask

**Decision:** `f=` carries one character per feature in a fixed order: `1` succeeded, `0` failed, `-` not tried. There are no separators, because every state is exactly one character.

**Alternatives considered:**

- _Single bitmask:_ `0` would mean not applicable, skipped and failed at once, so failure rates can't be computed.
- _Two bitmasks (attempted, succeeded):_ correct, but unreadable without arithmetic.
- _Key per feature (`hm=1;oc=1;…`):_ readable, but about 28 characters for five features, which doesn't fit the budget.
- _Base-3 integer:_ the most compact, but opaque.

The positional form reads at a glance (`f=11---`), costs 3 + N characters, and follows the precedent of the watch `t=` field (`watchSignalOrder`). A shorter string from an older dtwiz version means "this version didn't know the trailing features", which stays distinguishable from `-`.

The order lives in a single slice in `cmd/selfmonitoring.go` next to `watchSignalOrder`, with a comment that positions are add-only and never reordered or reused.

### Outcome recorder in `pkg/installer`

**Decision:** A small recorder in a new file `pkg/installer/install_telemetry.go`:

- `type Feature int` with constants `FeatureOtelConfig`, `FeatureHostMonitoring`, `FeatureRUM`, `FeatureSynthetic`, `FeatureRDSExtensions`.
- `RecordFeature(f Feature, ok bool)`: marks the feature as tried, succeeded or failed. The last call wins.
- `FeatureOutcomes() map[Feature]Outcome`: read by the cmd layer; features never recorded are not tried.
- `ResetInstallTelemetry()`: called by the cmd layer before each installer. It also resets the stopwatch.

It is guarded by a mutex because the AWS CloudFormation goroutine reports concurrently with the main thread.

Recording sites:

| Feature         | Site                                                                                                                    | Covers                           |
| --------------- | ----------------------------------------------------------------------------------------------------------------------- | -------------------------------- |
| OTel config     | `os.WriteFile` in collector plan execute (`collector.go`)                                                               | `otel`, `otel-collector`, `demo` |
| OTel config     | `writeConfig` (`update.go`), shared by both update paths                                                                | setup's `otel-update`            |
| OTel config     | the Dynatrace-collector update path when the config is unchanged but tenant prerequisites are applied (records success) | setup's `otel-update`            |
| Host Monitoring | inside `activateHostMonitoringExtension` (success at the end, failure at each early return)                             | all three OTel callers           |
| Host Monitoring | `InstallOneAgentV2` on the result of the installer command                                                              | `oneagent`, setup's OneAgent     |
| Host Monitoring | `InstallDocker` on the result of `docker run`                                                                           | `docker`                         |

**Alternatives considered:**

- _Return a result struct from every installer:_ changes every installer signature and caller, including setup and the tests. That's the same blast-radius argument the previous change used against callback parameters.
- _Change `activateHostMonitoringExtension` to return a bool:_ still needs threading through two or three call layers to reach the cmd layer, for no gain over recording at the source.

Recording at the source also means a feature is automatically not tried when the install fails before reaching that step, as the spec requires.

### Install stopwatch replaces `ExecutionStart`

**Decision:** A stopwatch in the same file, with these rules:

- **Start:** the install confirmation starts it. `confirmProceed` starts it when it isn't running yet and the user confirms; `ShouldProceed` goes through `confirmProceed`. For methods without a confirmation (fresh OneAgent, `docker`), the cmd layer calls `StartInstallTimer()` right before the installer, in both install and setup.
- **Pause:** a `confirmProceed` call while the stopwatch is running records a pause interval covering the time spent at the prompt. This fixes the virtualenv prompt and the OneAgent "already installed, update?" prompt without special cases.
- **Pre-confirmation prompts must not start it.** "Select another project?" (`otel.go`, `nodejs.go`) is asked before the install confirmation. Those two sites switch to a non-starting variant, `ConfirmQuestion`, so the time spent choosing a project isn't counted.
- **Stop:** `MarkInstallDone()` records the end, and the first call wins. When nothing marks it, the end is the moment the cmd layer reads the stopwatch after the installer returns.
- **Elapsed time** = end − start − the sum of pause intervals that started before the end. Clipping to the end keeps a past end time (AWS) correct.

Explicit end marks:

| Method                                                       | Mark                                                                                                                                                                                     |
| ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `azure`, `gcp` install; setup's `azure-update`, `gcp-update` | immediately before `WatchIngestCloudFromTimeWithEvent`                                                                                                                                   |
| `aws`                                                        | the CloudFormation goroutine records its completion time; the main thread records Lambda completion; after `wg.Wait()` the installer calls `MarkInstallDoneAt(max(cfnDone, lambdaDone))` |
| setup's `otel-update` (Dynatrace collector)                  | immediately before its `WatchIngest` call                                                                                                                                                |
| all others                                                   | none: the installer returns before the cmd layer starts the watch                                                                                                                        |

**Alternatives considered:**

- _Stop automatically when the watch starts:_ elegant, but wrong for AWS, where the watch starts while CloudFormation is still deploying.
- _Keep overwriting `ExecutionStart`:_ this is what causes the virtualenv bug.

### Wire format

`EventParams` gets two User-Agent-only fields, `Features` (`f=`) and `DurationS` (`d=`). They are emitted on `ist` events only, after the existing pairs, and omitted when empty. The body properties go through `ExtraProps`, like the watch timings.

**Feature outcomes, User-Agent `f=`:** one character per feature in the fixed order below, no separators.

| Character | Outcome   |
| --------- | --------- |
| `1`       | succeeded |
| `0`       | failed    |
| `-`       | not tried |

**Feature outcomes, body:** one property per feature, with the value `succeeded`, `failed` or `not_tried`. All five are always present together.

| Position in `f=` | Feature                                 | Body property                  |
| ---------------- | --------------------------------------- | ------------------------------ |
| 1                | OTel collector config written           | `install.otel_config_written`  |
| 2                | Host Monitoring enabled                 | `install.host_monitoring`      |
| 3                | RUM enabled (reserved)                  | `install.rum`                  |
| 4                | Synthetic Monitoring enabled (reserved) | `install.synthetic_monitoring` |
| 5                | RDS extensions enabled (reserved)       | `install.rds_extensions`       |

Examples for the spec scenarios:

| Scenario                                                            | `f=`              |
| ------------------------------------------------------------------- | ----------------- |
| OTel collector install, everything succeeds                         | `11---`           |
| Host Monitoring activation fails, install succeeds                  | `10---`           |
| Collector download fails after Host Monitoring activation           | `-1---`           |
| OneAgent install succeeds / fails                                   | `-1---` / `-0---` |
| Kubernetes, language-specific OTel, cloud methods, Azure/GCP update | `-----`           |
| OTel Collector update of a non-Dynatrace collector                  | `1----`           |

**Work time:**

- Body: `install.duration_ms`, in milliseconds and unclamped, matching the millisecond values of the watch body.
- User-Agent: `d=<seconds>`, in whole seconds rounded down and clamped to `9999` (about 2.8 hours). For example, a 3-hour install sends `d=9999` and `install.duration_ms: "10800000"`.

**Presence:** duration and features are sent exactly when the stopwatch was started. That single condition covers dry-run, cancellation and failure before confirmation.

**Length budget:** HAProxy captures 64 characters of the User-Agent. The worst case, `dtwiz/10.10.10;c=set;st=ist;s=awsl;er=ifl;f=-----;d=9999`, is 56 characters: a version with two digits per part, the longest method shortcode (4 characters), an error code, all five features and the clamped duration. Each future feature adds one character.

**Replaced v1.10.0 properties:**

| v1.10.0                                         | Replacement                                                                                                                                   |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `install.duration_s` (whole seconds)            | `install.duration_ms`; divide by 1000 to compare                                                                                              |
| `install.host_monitoring_enabled: "true"`       | `install.host_monitoring`; only `"succeeded"` means enabled                                                                                   |
| `install.otel_pipelines: "traces,metrics,logs"` | none; OTel installs always configure all three pipelines, so use `install.otel_config_written` to tell whether a collector config was written |

### One builder for both `ist` paths

**Decision:** `fireInstallEvent` (direct install) and `fireSetupInstallEvent` (setup) share one helper that adds duration and feature outcomes. The two keep their own rules for when they fire (direct installs also fire on cancellation; setup doesn't).

## Risks / Trade-offs

- [The v1.10.0 body keys disappear] → They existed for a single release, and the spec's REMOVED section documents the migration. Queries comparing across versions divide `install.duration_ms` by 1000.
- [A new prompt added mid-install that bypasses `confirmProceed`, such as a raw `bufio` read, would be counted as work time] → All prompts in `pkg/installer` go through `confirmProceed` today. Note this in the `ConfirmProceed` doc comment.
- [A new installer that runs the watch internally but forgets `MarkInstallDone`] → Its duration silently includes the watch. Mitigation: a unit test per cloud installer asserting the mark is set before the watch callback fires, plus a comment on the `WatchIngest*WithEvent` variants pointing to `MarkInstallDone`.
- [Package-level state and concurrency] → Same accepted debt as `ExecutionStart`. The mutex covers the one existing concurrent writer (the AWS goroutine); concurrent installs would still need a redesign.
- [Kubernetes duration includes up to 10 minutes of pod-readiness wait, unlike other methods] → Intended: work time runs until dtwiz's own verification finishes. Documented in the spec.
- [OneAgent and Docker report host monitoring from the installer result, so `f=-0---` duplicates `er=`] → Accepted for uniformity: host monitoring queries only need `f=`.

## Migration Plan

Telemetry only; no user-facing behavior changes. Ship in the next release with CHANGELOG entries under **Changed** listing the replaced body keys. Rollback is a revert: older and newer CLI versions report side by side, and queries can branch on the `version` property.
