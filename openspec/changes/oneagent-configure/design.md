# Design

## Context

The existing `InstallOneAgent` flow in `pkg/installer/oneagent.go` receives a `*client.ClassicClient` with auth already configured upstream — no token extraction at the installer layer.

## Goals / Non-Goals

**Goals:**

- Confirm that no token resolution or extraction is needed at the installer layer — credentials are already embedded in `c.Classic` before reaching the installer.

**Non-Goals:**

- Implementing a separate token resolution or extraction step in `pkg/installer/oneagent/`.
- Minting or exchanging tokens via any external API.

## Decisions

### 1. No token resolution needed at the installer layer

`validateCredentials` in `cmd/auth.go` already resolves which credential to use (access token preferred, platform token fallback) and returns `classicTok`. `setupClientFromCreds` embeds it into `c.Classic`'s resty HTTP client as the `Authorization` header.

`DownloadInstaller`, when implemented, will call `c.Classic.HTTP().R().Get(url)` — identical to how `downloadOneAgentInstaller(c *client.ClassicClient)` works in `oneagent.go`. The resty client is the right choice here (vs plain `net/http`) because the download hits the authenticated Dynatrace Classic API and benefits from the pre-configured auth header, retry policy, timeout, and `User-Agent`. The token is never extracted or passed explicitly.

### 2. Token confidentiality

Because the token is embedded in the resty client and never extracted to a variable in installer code, it cannot accidentally appear in log lines. The resty client redacts the `Authorization` header in verbose output via `sensitiveHTTPHeaders` in `pkg/client/client.go`.
