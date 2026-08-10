# Tasks: otel-host-monitoring-grail-routes

## 1. Route model and constants

- [ ] 1.1 In `pkg/installer/otel/`, add a new file (for example `grail_routes.go`) defining the three signal types (metrics, logs, spans), their documented matching-condition constants, and the target pipeline display name constant "OpenTelemetry Host Monitoring", with a source-URL comment pointing at the Smartscape on Grail docs. Include the per-signal schema pairing from `design.md`'s "Per-signal schema reference" table (`pipelineSchema`, `routingSchema`).
- [ ] 1.2 Define a `routingEntry` struct (`enabled`, `pipelineType`, `pipelineId`, `matcher`, `description`) and a routing-object wrapper (`value.routingEntries[]`) keeping them internal to the otel package.

## 2. Pipeline resolution

- [ ] 2.1 Add a resolver that, given an `installer.ExtensionClient` and a signal's pipeline schema, lists that schema (`Settings.ListObjects`, no `externalIds` filter) and returns the Settings **objectId** of the object owned by the OpenTelemetry Host Monitoring extension, identified by `strings.HasPrefix(obj.ExternalID, "com.dynatrace.extension.opentelemetry_")` (design.md Decision 2). Prefer `externalId` over `value.metadataList` `extension_name` for ownership detection: `externalId` is platform-assigned and gives a more unambiguous reference than extension-internal metadata. Do not use the `customId` or `externalId` string as the routing `pipelineId` (rejected: `400 Must be of type setting`); the objectId returned here is what routing entries require (task 3.3).
- [ ] 2.2 Return a distinct "not found" result (empty objectId, not an error) when no owned pipeline exists in the schema, so the caller can skip that signal safely.

## 3. Additive, idempotent reconciliation

- [ ] 3.1 Add a `ReconcileGrailRoutes(envURL, platformToken string, dryRun bool) error` (or equivalent) that builds an `installer.ExtensionClient`, resolves each signal's pipeline (task 2), and computes a per-signal plan: create / re-enable / already-exists / skipped.
- [ ] 3.2 For each signal, list the single `builtin:openpipeline.<signal>.routing` object (it may not exist yet; treat zero items as empty `routingEntries`, not an error) and scan `value.routingEntries` for an entry whose `pipelineId` equals the resolved pipeline objectId. If found and `enabled: true`, treat as already-satisfied (no-op). If found and `enabled: false`, set `enabled: true` on that entry only (leave `matcher`, `description`, `pipelineType`, and all other fields unchanged) and include it in the plan as "re-enable". Never modify any other field or entry.
- [ ] 3.3 For missing routes, append one new `routingEntry` (documented `matcher`, resolved pipeline **objectId** as `pipelineId`, `pipelineType: "custom"`, `enabled: true`, `description: "OpenTelemetry Host Monitoring"`) to `routingEntries`. Preserve all existing entries and their order. If the routing object already exists, PUT the whole object back through the dtctl `settings.Handler` with the object's `SchemaVersion` as `If-Match`, mirroring `pkg/installer/gcp/dtapi.go` line 196 (`SetHeader("If-Match", obj.SchemaVersion)`). If the routing singleton does not exist yet, POST-create it via `Settings.Create` with `value.routingEntries` set to the single new entry instead. Never modify or delete existing entries.
- [ ] 3.4 Parse and surface `constraintViolations` from the response body on write failure, reusing the helper pattern from the GCP/Azure installers (design.md Decision 2 shows the exact 400 body this guards against).
- [ ] 3.5 On any 401/403 from an OpenPipeline call, enrich the error with the schema/operation and the scope that normally covers it (`settings:objects:read` for reads, `settings:objects:write` for writes), per `design.md` Decision 6. Phrase it as a starting point, not a certain diagnosis: dtwiz cannot tell "token lacks the scope" apart from "token has the scope but its IAM policy restricts it to a schema list that excludes this schema" (see design.md Decision 6's limitation note). Follows the precedent in `pkg/installer/otel/collector.go`'s DQL scope hint and `pkg/installer/aws_lambda.go`'s named-scope 403 errors, adapted for this uncertainty. Errors that are not 401/403 (e.g. the 400 constraint violation from task 3.4) must pass through unchanged.

## 4. Preview, confirm, dry-run

- [ ] 4.1 Print the per-signal plan one line each (signal, create/re-enable/exists/skip). No separate confirmation prompt for routes: the existing `"Proceed with installation?"` prompt in `otel.go` already covers them. `ReconcileGrailRoutes` receives `dryRun bool` and returns early (after printing the plan) when it is true — no writes.
- [ ] 4.2 There is no route-only cancellation path. `ReconcileGrailRoutes` is only called after the user has already confirmed (or `--yes` was set); it never calls `ShouldProceed` or returns `ErrInstallCancelled`.

## 5. Wire into the install flow

- [ ] 5.1 In `pkg/installer/otel/otel.go`, call `ReconcileGrailRoutes(envURL, platformToken, dryRun)` after `collectorPlan.execute(...)` succeeds, only when `featureflags.IsEnabled(featureflags.Experimental)` is true.
- [ ] 5.2 Ensure the step never fails the overall install for a missing pipeline (skip) and that `errors.Is(err, installer.ErrInstallCancelled)` is treated as a clean skip. Only propagate genuine write/auth errors.
- [ ] 5.3 Confirm `WatchIngest` (which runs inside `cp.execute` via `verifyOtelInstall`) always completes before `ReconcileGrailRoutes` is invoked, and that the step is a no-op on the non-experimental path.

## 6. Tests

- [ ] 6.1 Unit-test the plan computation with a stubbed settings client: empty `routingEntries` -> entry appended for each signal; entry present and `enabled: true` -> no-op; entry present and `enabled: false` -> re-enabled (only the `enabled` field changes, all other fields and all sibling entries unchanged); mixed -> only missing/disabled signals acted on; user-broadened entry (same `pipelineId`, different `matcher`) -> recognized as existing, no duplicate. Assert existing entries survive the write unchanged (the PUT body contains prior entries + any additions/re-enables).
- [ ] 6.2 Unit-test that a missing "OpenTelemetry Host Monitoring" pipeline for one signal yields a skip for that signal, creates for the others, and returns success.
- [ ] 6.3 Unit-test the matching-condition constants match the documented strings exactly (metrics, logs, spans).
- [ ] 6.4 Unit-test that when the routing singleton object does not exist yet, the plan still resolves to create (with an empty routing objectId) and applying it calls create (POST) rather than update (PUT), with the correct schema and a single entry referencing the resolved pipeline objectId.
- [ ] 6.5 Unit-test `withGrailScopeHint` (task 3.5): nil passes through; a non-401/403 error is unchanged; a 401 or 403 gets the schema/operation and the normally-applicable scope appended (phrased as a hint, not a diagnosis) and still satisfies `errors.Is` against the underlying sentinel.
- [ ] 6.6 Test the experimental gate: with the flag off, no settings client is constructed and no route read/write occurs; with it on, reconciliation runs.
- [ ] 6.7 Test `--dry-run` computes and prints the plan but performs no writes and returns `nil`.

## 7. Docs and verification

- [ ] 7.1 Update `CHANGELOG.md` `[Unreleased]` with the Smartscape on Grail dynamic-route reconciliation, noting it ships behind `--experimental`.
- [ ] 7.2 `make test` and `make lint` pass.
- [ ] 7.3 Manually verify against a live tenant: `install otel --experimental --dry-run` shows the correct three-route plan; a real run creates the routes; a second run is a no-op; and with the extension inactive the routes are skipped and the install still succeeds. (The PoC pass already did this once against a live tenant, see `proposal.md`'s Impact section, but re-verify against the final implementation.)

## 8. Extension activation status in the install preview

- [x] 8.1 Add `installer.ExtensionClient.GetStatus(extensionName)` (`pkg/installer/extension_client.go`) reporting `ExtensionNotInstalled` / `ExtensionInstalledInactive` / `ExtensionInstalledActive` with a single `Extension.Get` call, instead of composing the existing `LatestExtensionVersion` + `IsExtensionActive` helpers (which would call the same endpoint twice). Unit-tested for all three states plus an API-error case.
- [x] 8.2 Add `buildExtensionActivationPreview(envURL, platformToken)` in `pkg/installer/otel/otel.go`, a thin wrapper returning `installer.ExtensionStatus` directly (no otel-local status type), and `printExtensionActivationPreview` to render it as a one-line section, ordered before the OpenPipeline route plan section (design.md Decision 7).
- [x] 8.3 Wire the preview into `InstallOtelCollectorWithProject`, gated identically to the route plan (`featureflags.Experimental` and non-empty `platformToken`); a failure prints a warning and does not block the rest of the preview or the install.
- [x] 8.4 Add the corresponding requirement to `specs/otel-host-monitoring-grail-routes/spec.md` ("Extension activation status shown in the install preview, before the route plan").

## 9. Rebuild the route plan before applying

- [x] 9.1 Fix `InstallOtelCollectorWithProject` (`pkg/installer/otel/otel.go`) to call `buildGrailPlans(ctx, grailC)` again right before applying, instead of reusing the `grailPlans` computed during the preview (before confirmation and before extension activation runs). Falls back to the preview snapshot if the rebuild call itself errors.
- [x] 9.2 Reword the `grailActionSkip` message (used by both `printGrailPlan` and `printGrailApplyResults` in `pkg/installer/otel/grail_routes.go`) from "activate the OpenTelemetry Host Monitoring extension first" to "re-run install otel once the extension is active", since the route step only ever runs when the same `--experimental` flag that gates it also gates dtwiz's own extension activation attempt; telling the user to activate it themselves was never accurate.
- [x] 9.3 Add `TestBuildGrailPlans_RebuildAfterPipelineAppears` (`pkg/installer/otel/grail_routes_test.go`) asserting that a second `buildGrailPlans` call against the same client reflects a pipeline that became available after the first call, guarding the fix in 9.1.
- [x] 9.4 Update `design.md` (new Decision 8), `proposal.md`, and `specs/otel-host-monitoring-grail-routes/spec.md` to describe the plan being re-evaluated before applying rather than fixed at preview time.
- [x] 9.5 Add `waitForGrailPipelines` (`pkg/installer/otel/grail_routes.go`): a bounded poll, using `installer.ExtensionActiveMaxAttempts`/`installer.ExtensionActiveRetryDelay`, for any one signal's pipeline to become listable. Called unconditionally from `InstallOtelCollectorWithProject` right before the plan rebuild (task 9.1), whether or not the extension was freshly installed this run: an already-listable pipeline satisfies the very first check at negligible cost, and a freshly hub-installed one (Hub installs are asynchronous) gets the bounded wait it needs, without the caller needing to track which case it is. A timeout is advisory (logged under `--debug`); the rebuild and skip-with-re-run-message path still apply.
- [x] 9.6 Unit-test `waitForGrailPipelines` for the immediate-success and bounded-timeout cases (`TestWaitForGrailPipelines_AlreadyPresent`, `TestWaitForGrailPipelines_TimesOut`).
- [x] 9.7 Split the `grailActionSkip` wording by context: `printGrailPlan` (the install preview, before extension activation has run) now says "pending — extension not active yet" instead of "skip", since nothing has been attempted at preview time and the plan is always re-evaluated before applying (task 9.1). `printGrailApplyResults` (the actual outcome after dtwiz has tried to install/activate/wait) keeps the "skip — pipeline not found (re-run install otel once the extension is active)" wording from task 9.2, since that's the only place a skip is ever a final decision.
