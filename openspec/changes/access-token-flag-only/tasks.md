## 1. Code (already implemented)

- [x] 1.1 `cmd/auth.go`: `accessToken()` returns only `accessTokenFlag`; drop the `os.Getenv("DT_ACCESS_TOKEN")` fallback
- [x] 1.2 `cmd/root.go`: update `--access-token` flag help and root `Long` text to state the token is opt-in and not read from env
- [x] 1.3 `pkg/installer/oneagent/download.go`: drop `DT_ACCESS_TOKEN` from the 401/403 error hint
- [x] 1.4 Docs: update `README.md` and `AGENTS.md`; add `Changed` entry to `CHANGELOG.md`

## 2. Tests

- [x] 2.1 `cmd/auth_test.go`: add `TestAccessToken` — flag set returns flag value; flag empty returns "" even when `DT_ACCESS_TOKEN` env var is set (use `t.Setenv`); restore `accessTokenFlag` after each case
- [x] 2.2 `cmd/auth_test.go`: add `TestGetDtEnvironment_AccessTokenFlagOnly` — with env + platform-token set and `DT_ACCESS_TOKEN` exported but the flag unset, `getDtEnvironment()` returns an empty `accessTok`; with the flag set it returns the flag value

## 3. Verify

- [x] 3.1 `make test` — all pass
- [x] 3.2 `make lint` — no new issues
- [x] 3.3 `openspec validate access-token-flag-only` passes
