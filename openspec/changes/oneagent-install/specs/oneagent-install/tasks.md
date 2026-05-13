# OneAgent Install Tasks

## 0. Investigate and Confirm Assumptions

Before implementing, review the design and spec documents to understand the requirements and constraints.

**Files:** `design.md`, `spec.md`

- [ ] 0.1 Read `design.md` and `spec.md` to understand token minting, download, and signature verification requirements
- [ ] 0.2 Identify and document any unclear assumptions about API endpoints or certificate handling
- [ ] 0.3 Review existing OneAgent installer code patterns in `pkg/installer/oneagent.go`
- [ ] 0.4 Confirm logging, error handling, and cross-platform behavior align with the specification

## 5. Download Installer + Linux Signature Verification

Use the minted token to download the installer. On Linux, verify the signature against the published Dynatrace root CA.

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/oneagent.go` (extend or factor out shared download helper), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1)

### Part A — Download

- [ ] 5.1 Implement `DownloadInstaller(c *client.ClassicClient, mintedToken string, env Environment) (string, error)` that streams the installer to a temp file
- [ ] 5.2 Use the minted token in the request `Authorization` header (override the client's default token for this single request)
- [ ] 5.3 Select OS/arch from `env` (Linux x86 / Linux arm / Windows x86) to populate the installer-type path segment (Windows path handling is in Task 11)
- [ ] 5.4 Set temp file permissions to `0o700` on Unix; preserve existing `0o755` on the executable bit when running
- [ ] 5.5 Unit tests with `httptest.Server`: streaming download, 401 response, malformed body
- [ ] 5.5a Emit `logger.Debug("downloading installer", "url", downloadURL, "os", env.OS, "arch", env.Arch)` at the start of the request; do NOT include the token or `Authorization` header in any log field
- [ ] 5.5b Emit `logger.Verbose("installer downloaded", "path", tmpFile.Name(), "size_bytes", n)` after streaming completes

### Part B — Linux signature verification

- [ ] 5.6 Implement `VerifyInstallerSignature(env Environment, installerPath string, skip bool) error`
- [ ] 5.7 Return nil when `skip == true` or `env.OS != "linux"`
- [ ] 5.8 On Linux: locate `openssl` via `exec.LookPath`; return `"openssl is required to verify the installer signature. Install openssl or pass --no-verify-signature to skip."` if missing
- [ ] 5.9 Download `https://ca.dynatrace.com/dt-root.cert.pem` to a temp file using the resty client (no token needed)
- [ ] 5.10 Run the documented `openssl cms -verify` pipeline; on non-zero exit, return a wrapped error including stderr
- [ ] 5.11 Unit tests: skip flag honored, non-Linux skip, missing openssl error, mock pipeline success/failure (use fake `openssl` in `$PATH` via `t.Setenv`)
- [ ] 5.12 Emit `logger.Debug("openssl lookup", "path", p, "found", err == nil)` after the `exec.LookPath` call
- [ ] 5.13 Emit `logger.Debug("fetching dynatrace root ca", "url", caURL, "path", certPath)` before/after the CA download
- [ ] 5.14 On success, emit `logger.Verbose("installer signature verified")`
- [ ] 5.15 On failure, emit `logger.Debug("signature verification failed", "exit_code", code, "stderr", stderrCaptured)` before returning the wrapped error

---

## 6. Build + Execute Install Command

Build the OS-specific install command from `AgentConfig` and execute it (or preview under `--dry-run`).

**Files:** `pkg/installer/oneagent_v2.go` (extend — scaffolded in Task 1), `pkg/installer/sudo_unix.go` / `_windows.go` (reuse), `pkg/installer/oneagent_v2_test.go` (extend — scaffolded in Task 1)

### Part A — Build command

- [ ] 6.1 Implement `BuildInstallCommand(env Environment, cfg AgentConfig, opts InstallOptions, installerPath string) ([]string, error)`
- [ ] 6.2 Windows: emit `{installerPath, --quiet?, --set-monitoring-mode=<cfg.MonitoringMode>, --set-app-log-content-access=<cfg.AppLogContentAccess>, --set-host-group=<opts.HostGroup>?}` (Windows `--quiet` MUST be the first flag; Windows-specific implementation in Task 11)
- [ ] 6.3 Linux: emit `{/bin/sh, installerPath, --set-server=<apiURL>, --set-monitoring-mode=..., --set-app-log-content-access=..., --set-host-group=<opts.HostGroup>?}`; prepend `sudo` when `needsSudo()` is true
- [ ] 6.4 Unit tests covering both OS branches, `--monitoring-mode` override, host-group present/absent, `--quiet` flag ordering on Windows
- [ ] 6.4a Emit `logger.Debug("built install command", "argv", argv)` once the argv slice is final — the minted token is not in argv, so this is safe

### Part B — Execute command

- [ ] 6.5 Implement `ExecuteInstallCommand(argv []string, dryRun, quiet bool) (int, error)`
- [ ] 6.6 Dry-run: print `Command: <argv joined>`, return `(0, nil)`, do NOT shell out
- [ ] 6.7 Execute: stream stdout/stderr when `quiet == false`; capture when `quiet == true`
- [ ] 6.8 Return the subprocess exit code alongside any wrapping error; non-zero exit code is an error with the installer output included
- [ ] 6.9 Unit tests: dry-run produces no subprocess, executor returns exit code, stderr captured on failure
- [ ] 6.10 Emit `logger.Debug("executing installer", "argv", argv)` immediately before spawning the subprocess (NOT in the dry-run branch)
- [ ] 6.11 Emit `logger.Verbose("installer exited", "exit_code", code, "duration", time.Since(start))` after the subprocess returns
