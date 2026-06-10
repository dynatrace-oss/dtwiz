## Context

Credentials are resolved in `cmd/auth.go`. `accessToken()` currently returns the `--access-token` flag value, falling back to `os.Getenv("DT_ACCESS_TOKEN")`. `validateCredentials()` validates the platform token via DQL (hard requirement) and, when an explicit access token is set (`accessTok != "" && accessTok != platformTok`), returns it as the Classic API token without probing; otherwise it uses the platform token for API calls. `dtwiz status` renders the Access Token row only when `accessToken() != ""`.

The problem is the env-var fallback: a stale `DT_ACCESS_TOKEN` in the shell is indistinguishable from an intentional one and silently changes which token authenticates Classic API calls.

## Goals / Non-Goals

**Goals:**
- Access-token auth activates only via an explicit, per-invocation signal (`--access-token`).
- A leftover `DT_ACCESS_TOKEN` env var can never activate access-token auth.
- Behavior when no access token is supplied is unchanged: platform token used for Classic API calls.

**Non-Goals:**
- No access-token content validation in the setup/install path. The token stays trusted-on-use there, exactly as today.
- No change to platform-token DQL validation, URL families, or `AuthHeader()` scheme selection.
- No new flag is introduced.

## Decisions

**The value-bearing `--access-token` flag is the opt-in signal — not a separate boolean.** A flag that carries a token value is inherently explicit (you cannot "leftover" a value typed on the command line this invocation), and it carries both intent and a usable value. A separate boolean (e.g. `--access-token-authentication`) was rejected: it would split intent from value, create ambiguous states (boolean without token, token without boolean), and still leave the token sourced from the banned env var.

**Implementation is a one-line behavior change:** `accessToken()` returns only `accessTokenFlag`, dropping the `os.Getenv("DT_ACCESS_TOKEN")` fallback. Everything downstream already keys off `accessToken()` being empty, so `validateCredentials()` and the `status` Access Token row require no logic changes — only the source narrows.

## Risks / Trade-offs

- **[Breaking change for env-var-only users (e.g. CI)]** → Documented in CHANGELOG and README; migration is mechanical: `--access-token "$DT_ACCESS_TOKEN"`. Falling through to platform-token auth is the intended, safer default, not a silent failure.
- **[Dead "not set" hint branch in `status`]** → `printCredentialStatus` for the access token is only reached when the flag is set, so its `DT_ACCESS_TOKEN` "not set" hint is now unreachable. Left as-is (harmless); not pruned to keep the change minimal.

## Migration Plan

Ship in the next release. Rollback is restoring the `os.Getenv` fallback and prior doc strings — no persisted state or data migration.
