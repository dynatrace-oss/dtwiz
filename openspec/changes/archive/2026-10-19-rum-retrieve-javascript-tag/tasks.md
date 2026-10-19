# Tasks

## 1. Package setup

- [x] 1.1 Ensure `pkg/installer/otel/rum/` exists (created by `otel-rum-detection`; create it here
      if that change has not landed yet) and create `tag.go` in package `rum`.

## 2. Client

- [x] 2.1 Define a `rumClient` interface with `fetchJavaScriptTag(ctx context.Context, frontendID string) (string, error)`.
- [x] 2.2 Implement `sdkRumClient` wrapping `*installer.ExtensionClient`, matching the shape of
      `sdkGrailClient` (`pkg/installer/otel/grail_routes.go`) and `sdkOTLPMetricDimensionsClient`
      (`pkg/installer/otel/otel_metric_dimensions.go`).
- [x] 2.3 Implement `newSDKRumClient(envURL, platformToken string) (*sdkRumClient, error)` backed by
      `installer.NewExtensionClient`.

## 3. Tag retrieval

- [x] 3.1 Implement `(c *sdkRumClient) fetchJavaScriptTag(ctx context.Context, frontendID string) (string, error)`:
      `GET /platform/rum/v1/manualInsertionTags/javaScriptTag/{frontendID}` (URL-escape `frontendID`).
- [x] 3.2 On HTTP 404, return `"", &installer.InstallFailedError{Step: "rum_tag_fetch"}` — checked
      before the generic response-check call, mirroring `sdkOTLPMetricDimensionsClient.schemaExists`.
- [x] 3.3 On any other non-2xx or transport error, return a wrapped error via
      `httpclient.CheckResponse` (generic failure — not the dedicated not-found case).
- [x] 3.4 On success, return `strings.TrimSpace(resp.String())`.

## 4. Tests

- [x] 4.1 Create `pkg/installer/otel/rum/tag_test.go` using `httptest.NewServer`; no real network
      calls. Define a shared test-local constant for the mocked FRONTEND ID (not referenced by any
      production code — there is no production call site yet).
- [x] 4.2 Test case: success — server returns 200 with a `text/plain` body (including surrounding
      whitespace); `fetchJavaScriptTag` returns the trimmed tag text.
- [x] 4.3 Test case: unknown entity — server returns 404; `fetchJavaScriptTag` returns an error
      that `errors.As` resolves to `*installer.InstallFailedError` with `Step == "rum_tag_fetch"`.
- [x] 4.4 Test case: server error (500) — `fetchJavaScriptTag` returns an error that is NOT an
      `*installer.InstallFailedError` with the `rum_tag_fetch` step (covers the "not misreported as
      not-found" scenario).
- [x] 4.5 Test case: transport-level failure (e.g. server closed/unreachable) — `fetchJavaScriptTag`
      returns an error, also not classified as the dedicated not-found case.
- [x] 4.6 Run `go test ./pkg/installer/otel/rum/...` and confirm all cases pass.
- [x] 4.7 Run `make lint` to confirm no golangci-lint issues in the new file.
