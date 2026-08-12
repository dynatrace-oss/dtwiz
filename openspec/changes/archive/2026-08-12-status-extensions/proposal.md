# Proposal: Status Extensions

## Why

`dtwiz status` today only validates that credentials are configured and reachable at the environment level. It gives no signal about whether the Dynatrace Extensions APIs are operational. Operators deploying extensions-based monitoring need to verify both the Classic Extensions API (`/api/v2/extensions`) and the Platform Extensions API (`/platform/extensions/v2/extensions`) before relying on them. Separately, all installer code that talks to Dynatrace uses ad-hoc `resty` clients created inline — making it hard to apply consistent retry, timeout, and debug-logging behaviour across the CLI.

## What Changes

- New centralized HTTP client (`pkg/client/client.go`) with `ClassicClient` and `PlatformClient`, constructed via `client.New()` and shared by commands that need it
- `--extensions` flag on `dtwiz status` that probes both Extensions APIs and reports reachability and package counts

## Capabilities

### New Capabilities

- `status --extensions`: Probes Classic and Platform Extensions APIs and prints reachability + count of installed extensions/packages

### Modified Capabilities

- `dtwiz status`: Gains an optional `--extensions` section rendered below the token validation block

## Impact

- **New files**: `pkg/client/client.go`, `pkg/client/client_test.go`
- **Modified files**: `cmd/status.go`, `cmd/root.go`
- **Dependencies**: none new — uses existing `go-resty/resty/v2`
- **APIs used**: `GET /api/v2/extensions` (Classic), `GET /platform/extensions/v2/extensions` (Platform)
- **Auth**: both sub-clients use the platform token by default; an explicit access token (opt-in via `--access-token`) takes precedence for Classic API calls when provided
