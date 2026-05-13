# OneAgent Configure Tasks

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [ ] 0.1 Read `design.md` and `spec.md` to understand agent configuration and token minting requirements
- [ ] 0.2 Identify and document any unclear assumptions about API scopes or monitoring mode defaults
- [ ] 0.3 Review existing token-minting patterns and security considerations in the codebase
- [ ] 0.4 Confirm logging and error handling practices align with the specification

## 4. Mint Short-lived Installer Token (Mandatory)

Mint an `InstallerDownload`-scoped token for the duration of this install. There is no fallback to the user's long-lived token.

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1)

- [ ] 4.1 Implement `MintInstallerToken(c *client.ClassicClient) (string, error)` posting to `/api/v2/tokens` with `{name: "dtwiz-oneagent-installer", scopes: ["InstallerDownload"], expiresIn: {value: 1, unit: "HOURS"}}`
- [ ] 4.2 Extract `token` from the response body; return it without logging the value
- [ ] 4.3 On any non-2xx response, return a wrapped error that includes the API status and response body (no token value)
- [ ] 4.4 On network failure, return a wrapped error with guidance to check `--environment`/proxy settings
- [ ] 4.5 Unit tests: happy path (extracts token), 403 missing-scope (`tokens.write`), 5xx, network error
- [ ] 4.6 Audit logs: confirm the token value never reaches `logger.Debug`/`logger.Info` output
- [ ] 4.7 Emit `logger.Debug("minting installer token", "url", reqURL, "scopes", []string{"InstallerDownload"}, "expires_in", "1h")` before the request
- [ ] 4.8 On 2xx response, emit `logger.Debug("installer token minted", "status", resp.StatusCode())` — status only, NEVER the body (which contains the token)
- [ ] 4.9 On non-2xx response, emit `logger.Debug("installer token mint failed", "status", resp.StatusCode(), "body", string(body))` — failure body is safe to log because it does not contain a token
- [ ] 4.10 Add unit test that captures stderr with `--debug` enabled and asserts the returned token value never appears in the captured output, across both success and failure paths
