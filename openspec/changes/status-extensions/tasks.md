# Tasks: Status Extensions

## 1. Centralized HTTP client (`cmd/client.go`)

- [x] 1.1 Define `Client`, `ClassicClient`, `PlatformClient` structs with `HTTP()` and `BaseURL()` accessors
- [x] 1.2 Implement `NewHTTPClient()` — reads credentials via `environmentHint()`, `accessToken()`, `platformToken()`; returns errors with actionable messages when any are missing
- [x] 1.3 Implement `newRestyClient()` — shared resty client with `SetBaseURL`, `Authorization` header, retry on 429/5xx (3 retries, 1 s–10 s backoff), 6-minute timeout, `User-Agent: dtwiz/<version>`, `Accept-Encoding: gzip`
- [x] 1.4 Add pre-request hook for `-v`: log method + URL to stderr
- [x] 1.5 Add pre-request hook for `--debug`: additionally log headers (redact `authorization`, `x-api-key`, `cookie`, `set-cookie`) and response body

## 2. Tests (`cmd/client_test.go`)

- [x] 2.1 Unit tests for `NewHTTPClient()` error paths (missing env URL, missing access token, missing platform token)
- [x] 2.2 Verify `ClassicClient` base URL is the Classic API URL (no `.apps.`)
- [x] 2.3 Verify `PlatformClient` base URL is the Apps URL (with `.apps.`)

## 3. Status --extensions flag (`cmd/status.go`)

- [x] 3.1 Register `--extensions` bool flag on `statusCmd`
- [x] 3.2 Implement `printExtensionsStatus()` — calls `NewHTTPClient()`, probes `GET /api/v2/extensions` and `GET /platform/extensions/v2/extensions`, prints reachability + counts or error per line
- [x] 3.3 Call `printExtensionsStatus()` from `statusCmd.RunE` when `--extensions` is set

## 4. Verification

- [x] 4.1 `make test` — all tests pass
- [x] 4.2 `make lint` — no new lint issues
