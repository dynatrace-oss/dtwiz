# Tasks: Platform Token Primary Authentication

## 1. Auth layer (`cmd/auth.go`)

- [x] 1.1 Change `getDtEnvironment` to return `(envURL, accessTok, platformTok, err)` — raw tokens, no Classic API selection
- [x] 1.2 Add `checkClassicAccess(envURL, token string) error` — probes `GET /api/v2/settings/schemas`, returns error on 401/403
- [x] 1.3 Change `validateCredentials` to `(envURL, accessTok, platformTok string) (classicTok string, err error)` — validates platform token via DQL, probes Classic API, returns resolved classic token
- [x] 1.4 Add debug log lines for each auth path outcome (explicit access token used, platform token accepted, platform token rejected but proceeding)

## 2. Client setup (`cmd/root.go`)

- [x] 2.1 Add `setupClientFromCreds(envURL, classicTok, platformTok string)` helper
- [x] 2.2 Update `setupClient` to call new `getDtEnvironment` + `validateCredentials`

## 3. Command updates

- [x] 3.1 Update all `cmd/install.go` commands to use new 4-value `getDtEnvironment` + `validateCredentials`, pass `classicTok` to Classic API installers and `platformTok` to `WatchIngest`
- [x] 3.2 Update `cmd/setup.go` similarly
- [x] 3.3 Update `cmd/update.go` similarly
- [x] 3.4 Update `cmd/uninstall.go` similarly

## 4. HTTP client (`pkg/client/client.go`)

- [x] 4.1 Restore two-token `New(classicURL, platformURL, classicToken, platformToken string, verbosityLevel int)` — Classic client uses `authHeader(classicToken)`, Platform client always uses `"Bearer " + platformToken`

## 5. Tests

- [x] 5.1 Add `cmd/auth_test.go` with `TestCheckClassicAccess` (200/404/500 pass; 401/403 fail; network failure)
- [x] 5.2 Add `TestValidateCredentials` covering: platform token accepted, platform token rejected + access token fallback, platform token rejected + no access token, DQL failure
- [x] 5.3 Update `test/integration/setup.go` — `TEST_DT_PLATFORM_TOKEN` required, `TEST_DT_ACCESS_TOKEN` optional
- [x] 5.4 Update `test/e2e/otel_test.go` — use `env.ClassicToken` / `env.PlatformToken` instead of `env.Token`
- [x] 5.5 Fix `client.New` call sites in test files to pass both tokens
