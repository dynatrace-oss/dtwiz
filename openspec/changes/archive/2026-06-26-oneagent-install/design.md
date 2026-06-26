# Design

## Context

The existing `downloadOneAgentInstaller` helper in `pkg/installer/oneagent.go` streams the installer to a temp file via the resty client and shells out with positional install arguments — no signature verification, no structured command representation. This change covers Tasks 5 and 6 of the OneAgent PoC: download, Linux signature verification, command build, and command execution.

## Goals / Non-Goals

**Goals:**

- `DownloadInstaller` uses the credentials already embedded in the ClassicClient (set upstream by `validateCredentials`) — no token extraction in installer code. Confirmed by `oneagent-configure`.
- Linux signature verification is mandatory unless `--no-verify-signature` is passed; missing `openssl` is a hard error.
- `BuildInstallCommand` produces a complete, testable argv slice from typed inputs.
- `ExecuteInstallCommand` returns the subprocess exit code alongside any error; `--dry-run` previews without executing.

**Non-Goals:**

- Go-native CMS signature verification — `openssl cms -verify` against the published Dynatrace pipeline is the documented approach. A Go-crypto rewrite adds external dependency risk and divergence in PKCS#7 parsing.
- Windows-specific installer URL, `.exe` extension, and Authenticode verification — covered in `oneagent-windows`.
- Minting a separate installer-scoped token. Earlier drafts of this change planned a `MintInstallerToken` step; `oneagent-configure` eliminated that and standardised on the credential already embedded in `c.Classic`.

## Decisions

### 1. Download: reuse the ClassicClient credential

`DownloadInstaller` calls `c.HTTP().R().SetDoNotParseResponse(true).Get(path)`. The resty client already carries the correct `Authorization` header set upstream by `setupClientFromCreds`. The token is never extracted to a variable in installer code, so it cannot accidentally appear in log lines.

Download URL pattern:

- Linux x86: `/api/v1/deployment/installer/agent/unix/default/latest?arch=x86`
- Linux arm: `/api/v1/deployment/installer/agent/unix/default/latest?arch=arm`
- Windows: `/api/v1/deployment/installer/agent/windows/default/latest?arch=x86` (path-only; `.exe` extension and Windows execution flow are covered in `oneagent-windows`)

Temp file: `os.CreateTemp("", "dynatrace-oneagent-*.sh")` (or `*.exe` for Windows env) → `chmod 0o700` on Unix. During streaming, a `display.ProgressReader` wraps the response body and emits a `\r`-overwriting progress line to stderr (TTY-only, suppressed in CI/pipes, throttled to ≤10 Hz). On success, stderr is cleared and stdout outputs via `display.PrintStatusLine("installer", "<basename> (<size>)", display.ColorOK)`.

### 2. Linux signature verification: openssl subprocess

The Dynatrace Linux installer ships with a CMS detached signature. Verification pipeline (shell equivalent):

```bash
( echo 'Content-Type: multipart/signed; protocol="application/x-pkcs7-signature"; micalg="sha-256"; boundary="--SIGNED-INSTALLER"'; \
  echo ; echo '----SIGNED-INSTALLER' ; cat <installer> ) \
| openssl cms -verify -CAfile <dt-root.cert.pem>
```

In Go this is built without a shell: `cmd.Stdin = io.MultiReader(header, installerFile)` where `header` is the MIME prelude string and `installerFile` is the opened installer.

Steps:

1. `exec.LookPath("openssl")` — missing → hard error, no silent skip.
2. Download `https://ca.dynatrace.com/dt-root.cert.pem` to a temp file with a standalone `resty.New()` client (no auth needed). The URL is held in a package-level var so tests can override it.
3. Run pipeline via `exec.Command`; capture stderr.
4. Non-zero exit → error wrapping the openssl stderr.

On success: stdout outputs via `display.PrintStatusLine("signature", "Installer signature verified", display.ColorOK)`.

**Why openssl over Go-native CMS:** `openssl cms -verify` against the Dynatrace-published pipeline is the documented approach. A Go-crypto rewrite risks subtle PKCS#7 parsing divergence.

### 3. Build command: typed argv from AgentConfig

`BuildInstallCommand` returns `[]string` so callers can inspect, log, and test the full argv without string parsing.

Linux argv:

```bash
[sudo?, /bin/sh, <installerPath>, --set-server=<apiURL>, --set-monitoring-mode=<mode>, --set-app-log-content-access=<bool>, --set-host-group=<group>?]
```

`sudo` prepended when `needsSudo()` returns true. Windows argv covered in `oneagent-windows`.

### 4. Execute: exit code + streaming

`ExecuteInstallCommand` returns `(exitCode int, err error)`. Non-zero exit is wrapped as an error including the captured installer output. Streaming (`quiet == false`) lets the user see installer progress in real time. On success, stdout outputs via `display.PrintStatusLine("result", "Installer executed successfully", display.ColorOK)`. `--dry-run` prints the command via `fmt.Printf("Command: %s\n", ...)` and returns `(0, nil)` without spawning a process.

### 6. Existing agent detection — update offer

Before downloading the installer, `InstallOneAgentV2` checks whether OneAgent is already **installed** on this host via `installer.OneAgentInstalled()`. The check is best-effort: false negatives proceed to a normal install.

It deliberately detects *installation*, not a *running* service — a stopped-but-installed agent must still take the update path. Detection therefore leads with a filesystem check (instant, no subprocess, init-system-agnostic, and cleared on a clean uninstall) and falls back to a service/`PATH` probe only for custom install locations:

- **Unix:** `os.Stat("/opt/dynatrace/oneagent")` → fallback `exec.LookPath("oneagentctl")`
- **Windows:** `os.Stat("%ProgramFiles%\dynatrace\oneagent")` → `Get-Service "Dynatrace OneAgent"` (present in any state, not only `Running`) → fallback `exec.LookPath("oneagentctl")`

A `systemd is-active` / `Get-Service ... -eq Running` check is deliberately **not** used: it answers "running now", which misclassifies a stopped-but-installed agent as absent and is systemd-only on Linux.

Detection lives in `pkg/installer/oneagent/` as unexported `oneAgentInstalled()`, split across build-tagged files (`detect_unix.go`, `detect_windows.go`). It is called directly within the same package — no cross-package export needed.

When an existing installation is detected:

- **Normal mode:** prompt `OneAgent is already installed. Update it? [Y/n]` (honouring `installer.AutoConfirm`). Declining exits cleanly with no error, printing `update cancelled`.
- **Quiet mode (`--quiet`):** skip the prompt, proceed silently (unattended automation intent).
- **Dry-run:** change the plan header from `"Would install"` to `"Would update"`.

The underlying installer invocation is identical whether installing fresh or updating — OneAgent's own installer handles upgrades, token rotation, and config reconciliation (including a new `--set-server` URL) natively.

**Why not diff the current config:** Diffing the live agent config requires shelling out to `oneagentctl --get-server` and comparing — adding complexity for little gain since the installer is idempotent and fast. The simpler model is: detected → offer update → proceed.

### 7. Logging

| Stage | Level | Message | Keys |
|---|---|---|---|
| Download start | Debug | `"downloading installer"` | `url`, `os`, `arch` |
| Download done | Verbose | `"installer downloaded"` | `path`, `size_bytes` |
| openssl lookup | Debug | `"openssl lookup"` | `path`, `found` |
| CA cert fetch | Debug | `"fetching dynatrace root ca"` | `url`, `path` |
| Verify success | Verbose | `"installer signature verified"` | — |
| Verify failure | Debug | `"signature verification failed"` | `exit_code`, `stderr` |
| Build command | Debug | `"built install command"` | `argv` |
| Execute start | Debug | `"executing installer"` | `argv` |
| Execute done | Verbose | `"installer exited"` | `exit_code`, `duration` |

The token is never extracted from the resty client into a Go variable, so no log key can ever carry it. The `Authorization` header is also redacted by `sensitiveHTTPHeaders` in the resty pre-request hook.
