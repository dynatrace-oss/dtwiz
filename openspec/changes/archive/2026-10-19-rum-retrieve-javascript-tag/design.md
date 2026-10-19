# Design

## Context

RUM onboarding runs in three phases: detect, create the frontend entity and fetch
the JS tag, inject/print. `otel-rum-detection` covers phase 1 (detection) only. This change covers
the "fetch the JS tag" half of phase 2 — entity creation/discovery is a separate, not-yet-designed
concern and is assumed to already have produced a FRONTEND entity ID by the time this code runs.

The package already has a home: `pkg/installer/otel/rum/`, scaffolded by `otel-rum-detection` for
`detect.go`. This change adds a sibling file for tag retrieval.

## Goals / Non-Goals

**Goals:**

- Given a Smartscape FRONTEND entity ID, retrieve the current OpenTelemetry JavaScript tag text.
- Classify "entity ID not found" as a distinct, dedicated failure so it is reported accurately
  through the existing self-monitoring error taxonomy (`pkg/selfmonitoring/classify.go`), instead
  of falling through as a generic error.

**Non-Goals:**

- Resolving the FRONTEND entity ID (search or create). A placeholder constant stands in for it
  until a follow-on change adds real lookup/creation.
- The OneAgent-based tag variant (`oneAgentJavaScriptTag`). Only the OpenTelemetry tag
  (`javaScriptTag`) is implemented.
- Injecting or printing the tag (phase 3 — a later change).
- Wiring this capability into `dtwiz install otel`'s command flow. It is exercised only by its own
  test suite in this change, consistent with how `otel-rum-detection` landed detection without
  wiring it into the installer.

## Decisions

### Client pattern matches existing otel sub-clients

Every OTel sub-concern that calls a Dynatrace endpoint not covered by the dtctl SDK builds its own
small, independently-constructed client wrapping `installer.ExtensionClient` — see
`sdkGrailClient` (`grail_routes.go`) and `sdkOTLPMetricDimensionsClient`
(`otel_metric_dimensions.go`). Tag retrieval follows the same shape rather than threading a shared
client through the install flow:

```go
type rumClient interface {
    fetchJavaScriptTag(ctx context.Context, frontendID string) (string, error)
}

type sdkRumClient struct {
    *installer.ExtensionClient
}

func newSDKRumClient(envURL, platformToken string) (*sdkRumClient, error) {
    ec, err := installer.NewExtensionClient(envURL, platformToken)
    if err != nil {
        return nil, err
    }
    return &sdkRumClient{ExtensionClient: ec}, nil
}
```

### Response is plain text, not JSON

The API returns a `text/plain` payload containing the tag itself — no wrapper object to decode.
`fetchJavaScriptTag` returns `resp.String()` (trimmed), not a struct.

### 404 maps to `InstallFailedError`, not a new taxonomy category

The self-monitoring error taxonomy (`pkg/installer/errors.go`) is a fixed, small set of categories.
Its spec already defines `install_failed` + a `Step` attribute as the catch-all for a failure that
doesn't fit a more specific category (auth, config, dependency, network, platform) — see
"Unrecognised error" in `openspec/specs/selfmonitoring-error-taxonomy/spec.md`. A 404 here means
the given FRONTEND entity ID doesn't exist, which is exactly that case: not an auth failure, not a
transport-level network failure. It is surfaced as:

```go
return "", &installer.InstallFailedError{Step: "rum_tag_fetch"}
```

This is checked explicitly via `resp.StatusCode() == http.StatusNotFound` *before* the generic
`httpclient.CheckResponse` call, mirroring `sdkOTLPMetricDimensionsClient.schemaExists`'s pattern
of checking the specific status first. Any other non-2xx or transport error falls through to
`httpclient.CheckResponse`'s generic error, which `ClassifyError` catches via its `install_failed`
fallback (or a more specific category such as `auth_error`, if `CheckResponse` surfaces one).

Self-monitoring reporting itself requires no new code: `cmd/install.go` already wraps every
subcommand and calls `fireSelfMonitoringEventWithError` on whatever error is returned, which runs
it through `selfmonitoring.ClassifyError`. Returning the right typed error is sufficient.

### Mocked entity ID lives in the test suite, not production code

Per the "wiring" non-goal above, there is no production call site yet. The mocked FRONTEND ID is a
test-local constant used across table-driven test cases, not a constant embedded in install flow
code — there's nothing in the install flow to call it from yet.

## Key Paths

| Path | Role |
|---|---|
| `pkg/installer/otel/rum/tag.go` | `rumClient` interface, `sdkRumClient`, `newSDKRumClient`, `fetchJavaScriptTag` |
| `pkg/installer/otel/rum/tag_test.go` | Table-driven tests with `httptest.Server` |

## API Shape

```text
GET /platform/rum/v1/manualInsertionTags/javaScriptTag/{frontendId}
```

- Auth: `Bearer <platform-token>` (Platform URL family — `.apps.` domain).
- Success: HTTP 200, `Content-Type: text/plain`, body is the JavaScript tag text.
- Not found: HTTP 404 when `frontendId` does not identify an existing FRONTEND entity.

## Risks / Trade-offs

- Response content beyond plain text (e.g. future JSON envelope, or placement metadata) would
  require a shape change later. Accepted: the known response today is `text/plain`.
- A malformed or stale mocked FRONTEND ID in tests is not representative of real tenant IDs. Low
  risk since this change ships no production call site — nothing in a real install run depends on
  the mocked value.
