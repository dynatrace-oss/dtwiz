# Design

## Context

The existing `downloadOneAgentInstaller` helper in `pkg/installer/oneagent.go` streams the installer to a temp file using the user's `--access-token`. There is no signature verification and no structured command representation — the install command is assembled inline from positional parameters. This change covers Tasks 5 and 6 of the OneAgent PoC: download (with minted token), Linux signature verification, command build, and command execution.

## Goals / Non-Goals

**Goals:**

- `DownloadInstaller` authenticates with the minted token, not the user's long-lived token.
- Linux signature verification is mandatory unless `--no-verify-signature` is passed; missing `openssl` is a hard error.
- `BuildInstallCommand` produces a complete, testable argv slice from typed inputs.
- `ExecuteInstallCommand` returns the subprocess exit code alongside any error; `--dry-run` previews without executing.

**Non-Goals:**

- Go-native CMS signature verification — `openssl cms -verify` against the published Dynatrace pipeline is the documented approach. A Go-crypto rewrite adds external dependency risk and divergence in PKCS#7 parsing.
- Windows-specific installer URL, `.exe` extension, and Authenticode verification — covered in `oneagent-windows`.

## Decisions

### 1. Download: minted token override

`DownloadInstaller` overrides the resty client's default `Authorization` header for the single download request, using `Api-Token <mintedToken>`. The user's `--access-token` does not appear in the download request.

Download URL pattern:

- Linux x86: `/api/v1/deployment/installer/agent/unix/default/latest?arch=x86`
- Linux arm: `/api/v1/deployment/installer/agent/unix/default/latest?arch=arm`
- Windows: handled in `oneagent-windows`

Temp file: `os.CreateTemp("", "dynatrace-oneagent-*")` → `chmod 0o700` on Unix. On success, stdout prints `✓ <basename> (<size>)`.

Download URL pattern:

- Linux x86: `/api/v1/deployment/installer/agent/unix/default/latest?arch=x86`
- Linux arm: `/api/v1/deployment/installer/agent/unix/default/latest?arch=arm`
- Windows: handled in `oneagent-windows`

Temp file: `os.CreateTemp("", "dynatrace-oneagent-*")` → `chmod 0o700` on Unix. On success, stdout prints `✓ <basename> (<size>)`.

### 2. Linux signature verification: openssl subprocess

The Dynatrace Linux installer ships with a CMS detached signature. Verification pipeline (shell equivalent):

```bash
( echo 'Content-Type: multipart/signed; protocol="application/x-pkcs7-signature"; micalg="sha-256"; boundary="--SIGNED-INSTALLER"'; \
  echo ; echo '----SIGNED-INSTALLER' ; cat <installer> ) \
| openssl cms -verify -CAfile <dt-root.cert.pem>
```

Steps:

1. `exec.LookPath("openssl")` — missing → hard error, no silent skip.
2. Download `https://ca.dynatrace.com/dt-root.cert.pem` to a second temp file.
3. Run pipeline via `exec.Command`; capture stderr.
4. Non-zero exit → error wrapping the openssl stderr.

On success: `✓ Installer signature verified.` to stdout.

**Why openssl over Go-native CMS:** `openssl cms -verify` against the Dynatrace-published pipeline is the documented approach. A Go-crypto rewrite risks subtle PKCS#7 parsing divergence.

### 3. Build command: typed argv from AgentConfig

`BuildInstallCommand` returns `[]string` so callers can inspect, log, and test the full argv without string parsing.

Linux argv:

```bash
[sudo?, /bin/sh, <installerPath>, --set-server=<apiURL>, --set-monitoring-mode=<mode>, --set-app-log-content-access=<bool>, --set-host-group=<group>?]
```

`sudo` prepended when `needsSudo()` returns true. Windows argv covered in `oneagent-windows`.

### 4. Execute: exit code + streaming

`ExecuteInstallCommand` returns `(exitCode int, err error)`. Non-zero exit is wrapped as an error including the captured installer output. Streaming (`quiet == false`) lets the user see installer progress in real time. `--dry-run` prints `Command: <argv>` and returns `(0, nil)` without spawning a process.

### 5. Logging

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

Token values are never placed in argv and therefore safe to log at Debug.
