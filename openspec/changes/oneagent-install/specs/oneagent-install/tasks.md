# OneAgent Install Tasks

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [x] 0.1 Read `design.md` and `spec.md` to understand download, signature verification, and command construction requirements
- [x] 0.2 Identify and document any unclear assumptions about API endpoints or certificate handling
- [x] 0.3 Review existing OneAgent installer code patterns in `pkg/installer/oneagent.go`
- [x] 0.4 Confirm logging, error handling, and cross-platform behavior align with the specification

> **Findings (Task 0 outcome):**
>
> - The earlier draft of this change planned a minted installer-scoped token (`DownloadInstaller(c, mintedToken, env)`). `oneagent-configure` later eliminated minting and standardised on the credential already embedded in `c.Classic`. This change has been updated to match: `DownloadInstaller(c *client.ClassicClient, env Environment)`.
> - The `Environment` type referenced by Task 5/6 was planned in `oneagent-init` Task 1.5 and is defined in `pkg/installer/oneagent/oneagent.go` as `{OS, Arch, Supported, Reason}`.

## 5. Download Installer + Linux Signature Verification

Use the credentials embedded in the ClassicClient to download the installer. On Linux, verify the signature against the published Dynatrace root CA.

**Files:** `pkg/installer/oneagent/download.go` (extend), `pkg/installer/oneagent/oneagent_test.go` (extend)

### Part A — Download

- [x] 5.1 Implement `DownloadInstaller(c *client.ClassicClient, env Environment) (string, error)` that streams the installer to a temp file
- [x] 5.2 Use `c.HTTP().R().SetDoNotParseResponse(true).Get(path)` — the resty client carries the correct `Authorization` header set upstream by `setupClientFromCreds`; no token extraction in installer code
- [x] 5.3 Select OS/arch from `env` (Linux x86 / Linux arm / Windows x86) to populate the installer-type path segment (Windows execution flow is in Task 11)
- [x] 5.4 Set temp file permissions to `0o700` on Unix (covers both confidentiality and the executable bit needed at run time)
- [x] 5.5 Unit tests with `httptest.Server`: streaming download, 401 response, unsupported OS, URL/arch matrix
- [x] 5.5a Emit `logger.Debug("downloading installer", "url", downloadURL, "os", env.OS, "arch", env.Arch)` at the start of the request; the credential is never placed in a log field
- [x] 5.5b Emit `logger.Verbose("installer downloaded", "path", tmpFile.Name(), "size_bytes", n)` after streaming completes

### Part B — Linux signature verification

- [x] 5.6 Implement `VerifyInstallerSignature(env Environment, installerPath string, skip bool) error`
- [x] 5.7 Return nil when `skip == true` or `env.OS != "linux"`
- [x] 5.8 On Linux: locate `openssl` via `exec.LookPath`; return `"openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."` if missing
- [x] 5.9 Download `https://ca.dynatrace.com/dt-root.cert.pem` to a temp file using a standalone resty client (no token needed); URL is held in a package-level var so tests can override it
- [x] 5.10 Run the documented `openssl cms -verify` pipeline (header + installer streamed via `cmd.Stdin = io.MultiReader(...)`); on non-zero exit, return a wrapped error including stderr
- [x] 5.11 Unit tests: skip flag honored, non-Linux skip, missing openssl error, mock pipeline success/failure (fake `openssl` in `$PATH` via `t.Setenv`), CA-fetch failure aborts before openssl runs
- [x] 5.12 Emit `logger.Debug("openssl lookup", "path", p, "found", err == nil)` after the `exec.LookPath` call
- [x] 5.13 Emit `logger.Debug("fetching dynatrace root ca", "url", caURL, "path", certPath)` before the CA download
- [x] 5.14 On success, emit `logger.Verbose("installer signature verified")`
- [x] 5.15 On failure, emit `logger.Debug("signature verification failed", "exit_code", code, "stderr", stderrCaptured)` before returning the wrapped error

---

## 6. Build + Execute Install Command

Build the OS-specific install command from `AgentConfig` and execute it (or preview under `--dry-run`).

**Files:** `pkg/installer/oneagent/oneagent.go` (extend), `pkg/installer/sudo_unix.go` / `_windows.go` (reuse), `pkg/installer/oneagent/oneagent_test.go` (extend)

### Part A — Build command

- [ ] 6.1 Implement `BuildInstallCommand(env Environment, cfg AgentConfig, opts InstallOptions, installerPath string) ([]string, error)`
- [ ] 6.2 Windows: emit `{installerPath, --quiet?, --set-monitoring-mode=<cfg.MonitoringMode>, --set-app-log-content-access=<cfg.AppLogContentAccess>, --set-host-group=<opts.HostGroup>?}` (Windows `--quiet` MUST be the first flag; Windows-specific implementation in Task 11)
- [ ] 6.3 Linux: emit `{/bin/sh, installerPath, --set-server=<apiURL>, --set-monitoring-mode=..., --set-app-log-content-access=..., --set-host-group=<opts.HostGroup>?}`; prepend `sudo` when `needsSudo()` is true
- [ ] 6.4 Unit tests covering both OS branches, `--monitoring-mode` override, host-group present/absent, `--quiet` flag ordering on Windows
- [ ] 6.4a Emit `logger.Debug("built install command", "argv", argv)` once the argv slice is final — the credential is not in argv, so this is safe

### Part B — Execute command

- [ ] 6.5 Implement `ExecuteInstallCommand(argv []string, quiet bool) (int, error)` — no `dryRun` parameter; dry-run is checked beforehand in `InstallOneAgentV2` before this function is ever called
- [ ] 6.6 Execute: stream stdout/stderr when `quiet == false`; capture when `quiet == true`
- [ ] 6.7 Return the subprocess exit code alongside any wrapping error; non-zero exit code is an error with the installer output included
- [ ] 6.8 Unit tests: executor returns exit code, stderr captured on failure
- [ ] 6.9 Emit `logger.Debug("executing installer", "argv", argv)` immediately before spawning the subprocess
- [ ] 6.10 Emit `logger.Verbose("installer exited", "exit_code", code, "duration", time.Since(start))` after the subprocess returns
