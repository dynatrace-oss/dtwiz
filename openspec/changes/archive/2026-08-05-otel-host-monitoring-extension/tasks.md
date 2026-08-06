# Tasks: otel-host-monitoring-extension

## 1. ExtensionClient: ActivateExtension method

- [x] 1.1 Add `ActivateExtension(extensionName, version string) error` to `pkg/installer/extension_client.go` using `e.C.HTTP().R()` to call `POST /platform/extensions/v2/extensions/{name}/environment-configuration` with body `{"version": version}`
- [x] 1.2 Treat HTTP 409 (already active / conflict) as success in `ActivateExtension`, consistent with how `InstallExtension` handles 409
- [x] 1.3 Add unit test for `ActivateExtension` covering: success (2xx), idempotent (409), and API error cases

## 2. OTel install flow: extension activation step

- [x] 2.1 Define `const otelHostMonitoringExtension = "com.dynatrace.extension.opentelemetry"` in `pkg/installer/otel/otel.go`
- [x] 2.2 In `InstallOtelCollectorWithProject` (`pkg/installer/otel/otel.go`), after `ConfirmProceed()` returns `ok=true` and before `cp.execute()`, add the extension activation block gated on `featureflags.IsEnabled(featureflags.Experimental)`
- [x] 2.3 Activation block: call `installer.NewExtensionClient(envURL, platformToken)`, then `EnsureInstalled(otelHostMonitoringExtension)`, then `LatestExtensionVersion`, then `ActivateExtension` — on any error, call `logger.Debug(...)` and `fmt.Println` a warning, then continue
- [x] 2.4 Activation block must be skipped entirely when `dryRun == true` (already guarded by the `ConfirmProceed` block being unreachable, but add an explicit comment for clarity)

## 3. Tests

- [x] 3.1 Update `test/e2e/otel_test.go` `TestOTelHostMonitoring` to account for the extension activation attempt — verify the test still passes when the extension is already installed and active on the test tenant
- [x] 3.2 Add unit test for `InstallOtelCollectorWithProject` covering: experimental flag off (no extension calls), experimental flag on + extension already active (skips install), experimental flag on + extension freshly installed (calls activate), activation failure (advisory: install proceeds)

## 4. Verification

- [x] 4.1 Run `make build` and confirm no compile errors
- [x] 4.2 Run `make test` and confirm all tests pass
- [x] 4.3 Run `make lint` and confirm no new lint issues
- [x] 4.4 Manual verification: run `dtwiz install otel --experimental` against a test tenant and confirm the extension appears as active in the Dynatrace Hub after install completes
