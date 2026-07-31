# 1. Tests

- [x] 1.1 Add `TestCheckPlatformToken` in `cmd/auth_test.go`: table-driven, using `httptest.NewServer` + `credentialHTTPClient` swap, covering: 200 (no error), 401 (error contains "authentication failed"), 403 (error contains "insufficient permissions"), 500 (error contains status code)
- [x] 1.2 Add `TestCheckAccessToken` in `cmd/auth_test.go`: same pattern, same cases, asserting the "Access token:" prefix variants
- [x] 1.3 Run `make test` and confirm all new and existing tests pass
