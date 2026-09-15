# Tasks

## 1. Flush strategy — `pkg/selfmonitoring/selfmonitoring.go`

- [ ] 1.1 Add `StepFailed = "fai"` and `StepCancelled = "can"` constants alongside `StepInvoked` and `StepCompleted`.
- [ ] 1.2 Add `pendingEvent` struct with `classicURL string`, `token string`, `params EventParams` fields.
- [ ] 1.3 Add package-level `queue []pendingEvent` and `mu sync.Mutex`.
- [ ] 1.4 Add `Enqueue(classicURL, token string, params EventParams)`: sets `params.StepID` default if empty, then appends to queue under mutex.
- [ ] 1.5 Add `Flush(timeout time.Duration)`: drains queue under mutex, sends all pending events concurrently via goroutines + `sync.WaitGroup`, blocks until all complete or `context.WithTimeout` fires — silent on timeout.

## 2. Flush strategy — `cmd/selfmonitoring.go`

- [ ] 2.1 In `fireSelfMonitoringEvent`: move `getDtEnvironment()` call out of the goroutine to the top of the function (before feature flag check is fine; after is cleaner — keep guard at top). Remove the goroutine. Call `selfmonitoring.Enqueue(installer.APIURL(envURL), platformTok, params)` on success.
- [ ] 2.2 Add `fireSelfMonitoringEventWithError(params selfmonitoring.EventParams, err error)`: calls `installer.ClassifyError(err)`, sets `params.Err = string(errType)`, merges attrs into `params.ExtraProps`, then calls `fireSelfMonitoringEvent(params)`.

## 3. Flush strategy — `cmd/root.go`

- [ ] 3.1 In `Execute()`: change from `if err := rootCmd.Execute(); err != nil { os.Exit(1) }` to: call `rootCmd.Execute()`, then `selfmonitoring.Flush(200 * time.Millisecond)`, then `os.Exit(1)` if err non-nil.

## 4. Error taxonomy — `pkg/installer/errors.go` (new file)

- [ ] 4.1 Define `ErrorType string` type and constants: `ErrTypeUserCancelled`, `ErrTypeAuthError`, `ErrTypeConfigError`, `ErrTypeDependencyMissing`, `ErrTypeNetworkError`, `ErrTypeInstallFailed`, `ErrTypePlatformUnsupported`.
- [ ] 4.2 Add `ErrPlatformUnsupported = errors.New("platform not supported")` sentinel.
- [ ] 4.3 Add typed error structs: `AuthError{Reason string}`, `ConfigError{MissingFields []string}`, `DependencyMissingError{Name string}`, `NetworkError{Reason, URL string}`, `InstallFailedError{Step string}`. Each implements `error` with a human-readable `Error() string`. Each implements `Unwrap() error` returning `nil` (they are leaf errors, not wrappers).
- [ ] 4.4 Add `ClassifyError(err error) (ErrorType, map[string]string)`: walks error chain in priority order (see design), returns type + additional attributes map. Returns `(ErrTypeInstallFailed, nil)` for unrecognised errors.

## 5. Typed errors — `cmd/auth.go`

- [ ] 5.1 In `getDtEnvironment()`: when `DT_ENVIRONMENT` is missing, return `&installer.ConfigError{MissingFields: []string{"DT_ENVIRONMENT"}}`. When `DT_PLATFORM_TOKEN` is also missing in the same call, include both in a single `ConfigError`. Preserve existing display text as the `Error()` string by wrapping: `fmt.Errorf("<existing message>: %w", &installer.ConfigError{...})`.
- [ ] 5.2 In `checkPlatformToken()`: map HTTP 401 → `&installer.AuthError{Reason: "authentication_failed"}`, 403 → `&installer.AuthError{Reason: "invalid_token"}`, network failure or unexpected status → `&installer.NetworkError{Reason: "environment_not_reachable", URL: appsURL}`. Wrap with `fmt.Errorf` to preserve display text.
- [ ] 5.3 In `checkAccessToken()`: same mapping as 5.2 where applicable (401, network failure).

## 6. Typed errors — platform-unsupported

- [ ] 6.1 In `pkg/installer/oneagent/oneagent.go`: replace the three `fmt.Errorf("...not supported on %s...")` returns with `fmt.Errorf("<existing message>: %w", installer.ErrPlatformUnsupported)`.
- [ ] 6.2 In `pkg/installer/otel/collector.go`: replace `fmt.Errorf("unsupported OS for OTel Collector: %s", runtime.GOOS)` with `fmt.Errorf("unsupported OS for OTel Collector: %s: %w", runtime.GOOS, installer.ErrPlatformUnsupported)`.

## 7. Typed errors — dependency missing (~16 sites)

Replace each `exec.LookPath` / `ExecLookPath` "not found" failure with a `DependencyMissingError` wrapper. Preserve the existing human-readable error message using `fmt.Errorf("<existing message>: %w", &installer.DependencyMissingError{Name: "<binary>"})`.

- [ ] 7.1 `pkg/installer/aws.go` — `aws` binary check.
- [ ] 7.2 `pkg/installer/docker.go` — `docker` binary check.
- [ ] 7.3 `pkg/installer/azure/preflight.go` — `az` binary check (via `execLookPath`).
- [ ] 7.4 `pkg/installer/gcp/preflight.go` — `gcloud` binary check (via `execLookPath`).
- [ ] 7.5 `pkg/installer/otel/java.go` — `java` binary check.
- [ ] 7.6 `pkg/installer/otel/java_process.go` — `java`, `mvn`, `gradle`, `javac`, `jps` binary checks.
- [ ] 7.7 `pkg/installer/otel/golang.go` — `go` binary check.
- [ ] 7.8 `pkg/installer/otel/nodejs.go` — `node`, `npm` binary checks.
- [ ] 7.9 `pkg/installer/otel/collector.go` — `codesign` binary check.
- [ ] 7.10 `pkg/installer/otel/demo.go` — `brew`, `winget` binary checks.
- [ ] 7.11 `pkg/installer/otel/otel.go` — runtime binary name checks.
- [ ] 7.12 `pkg/installer/kubernetes/helm_install.go` — `helm`, `winget` binary checks.
- [ ] 7.13 `pkg/installer/oneagent/detect_unix.go` — `oneagentctl` binary check.
- [ ] 7.14 `pkg/installer/oneagent/detect_windows.go` — `oneagentctl` binary check.
- [ ] 7.15 `pkg/installer/oneagent/install_agent.go` — `sudo` binary check.
- [ ] 7.16 `pkg/installer/oneagent/verify.go` — `openssl` binary check.

## 8. Terminal events — `cmd/install.go`

- [ ] 8.1 For every installer call site (oneagent, kubernetes, otel, otel-collector, aws, aws-lambda, azure, gcp, docker, demo): fire `fireSelfMonitoringEventWithError(buildEventParams(cmd, selfmonitoring.StepCancelled), err)` before `return nil` on `ErrInstallCancelled`; fire `fireSelfMonitoringEventWithError(buildEventParams(cmd, selfmonitoring.StepFailed), err)` before `return err` on other errors; fire `fireSelfMonitoringEvent(buildEventParams(cmd, selfmonitoring.StepCompleted))` on success.

## 9. Terminal events — `cmd/setup.go`, `cmd/update.go`, `cmd/uninstall.go`

- [ ] 9.1 `cmd/setup.go`: apply the same terminal event pattern at every return in the setup `RunE` (analyze step, recommend step, install step).
- [ ] 9.2 `cmd/update.go`: apply terminal event pattern at every return in update subcommand `RunE` handlers.
- [ ] 9.3 `cmd/uninstall.go`: apply terminal event pattern at every return in uninstall subcommand `RunE` handlers.

## 10. Terminal events — auth/config errors

- [ ] 10.1 Verify that `getDtEnvironment()` and `validateCredentials()` failures, which return early from `RunE`, are handled by the existing terminal event pattern from tasks 8 and 9 (they return an error from `RunE` like any other error, so the same `fireSelfMonitoringEventWithError` + `StepFailed` call covers them).

## 11. Tests

- [ ] 11.1 Unit-test `ClassifyError`: table-driven test covering each of the 8 error types (including fallback), verifying the returned `ErrorType` constant and `map[string]string` attributes.
- [ ] 11.2 Unit-test `Flush`: verify that all enqueued events are sent, that timeout is respected (no block beyond timeout), and that the queue is cleared after flush.
- [ ] 11.3 Unit-test `fireSelfMonitoringEventWithError`: verify `params.Err` is set to the classified type and `ExtraProps` contains the expected attributes.
- [ ] 11.4 Run `go test ./pkg/installer/... ./pkg/selfmonitoring/... ./cmd/...` and confirm all pass.
- [ ] 11.5 Run `make lint` and confirm no new issues.
