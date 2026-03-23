# dtwiz Prototype Takeover Checklist

This checklist is for validating the **current dtwiz prototype** against the epic *“CLI Data Onboarding (PRODUCT-16599)”* during handover.  
All items are phrased so you can **actually verify them in code or by running the CLI**.

---

## 1. Repository & Build State

- [ ] **Go module & build**
  - [ ] `go build ./...` succeeds on your machine.
  - [ ] The Go module path can be set to `github.com/dynatrace-oss/dtwiz` without conflicts.

- [ ] **Open source readiness**
  - [ ] A license file is present.
  - [ ] `README.md` exists and contains:
    - [ ] At least one example command (e.g. `dtwiz setup` or similar).
    - [ ] Short explanation of what the tool does.
  - [ ] `CONTRIBUTING.md` exists or a TODO is present referencing dtctl’s CONTRIBUTING as template.

- [ ] **CI basics**
  - [ ] A CI configuration file exists (e.g., GitHub Actions, GitLab, etc.).
  - [ ] CI runs `go test ./...` (or equivalent).
  - [ ] CI does **not** run integration tests (integration tests are either separate or clearly marked).

- [ ] **GoReleaser**
  - [ ] `.goreleaser.yml` (or equivalent) exists.
  - [ ] Running GoReleaser locally (or dry-run) completes without fatal errors.

---

## 2. Binary & Command Surface

- [ ] **Binary name**
  - [ ] The compiled binary is named `dtwiz` (or there is a single, trivial rename step).

- [x] **Top-level commands exist**
  Run `dtwiz --help` and verify:
  - [x] `analyze`
  - [x] `recommend`
  - [x] `setup`
  - [x] `status`
  - [x] `version`
  - [x] `install`
  - [x] `uninstall`

- [x] **Version command**
  - [x] `dtwiz version` exits with code 0.
  - [x] `dtwiz version` prints a non-empty version string or commit reference.

---

## 3. Authentication & Tokens

- [x] **Global flags present**
  Run `dtwiz --help` and confirm:
  - [x] `--environment` is listed.
  - [x] `--access-token` is listed.
  - [x] `--platform-token` is listed (even if not fully used yet).

- [ ] **Environment variables supported**
  - [ ] Code or docs show support for:
    - [ ] `DT_ENVIRONMENT`
    - [ ] `DT_ACCESS_TOKEN`
    - [ ] `DT_PLATFORM_TOKEN`

- [ ] **Error handling**
  With no token provided:
  - [ ] Commands that require Dynatrace access (e.g. `install` flows) fail with a clear error about missing token.
  With an invalid token:
  - [ ] Command fails with a clear error message (not just a generic HTTP error).

---

## 4. `dtwiz analyze`

- [x] **Command runs without credentials**
  - [x] `dtwiz analyze` exits with 0 in a typical local environment (with or without Dynatrace credentials set).

- [ ] **OS & architecture detection**
  - [ ] Output includes OS name (e.g., Linux/macOS/Windows).
  - [ ] Output includes architecture (e.g., amd64/arm64).

- [ ] **Kubernetes detection**
  In an environment with `kubectl` configured to a cluster:
  - [ ] `dtwiz analyze` output indicates that a Kubernetes cluster is detected.

- [ ] **Docker detection**
  With Docker running:
  - [ ] `dtwiz analyze` indicates Docker is present.
  - [ ] It shows either a container count or a clear “Docker is running” indicator.

- [ ] **Cloud credentials detection**
  In an environment with credentials configured:
  - [ ] AWS: `dtwiz analyze` shows AWS as detected (when AWS creds are present).
  - [ ] Azure: `dtwiz analyze` shows Azure as detected (when Azure creds are present).
  - [ ] GCP: `dtwiz analyze` shows GCP as detected (when GCP creds are present).

- [ ] **Existing instrumentation detection**
  In an environment where each exists:
  - [ ] OneAgent installed: `dtwiz analyze` indicates OneAgent presence.
  - [ ] OTel Collector installed: `dtwiz analyze` indicates OTel Collector presence (and, if implemented, its config path).

- [ ] **Runtime / daemon checks** (where you can simulate)
  - [ ] When a Java process is running, output includes a hint that Java is detected (same for Node.js, Python, Go if implemented).
  - [ ] When common daemons (e.g., nginx, PostgreSQL) are running, they are listed (if implemented).

- [ ] **Timeouts**
  - [ ] No single analyze run stalls for “minutes” on a broken environment; probes are clearly bounded (you can see timeouts in code or behavior).

- [ ] **Optional `--json`**
  - [ ] If `dtwiz analyze --json` exists:
    - [ ] It outputs valid JSON (parseable).
  - [ ] If not implemented:
    - [ ] There is no `--json` flag for `analyze`.

---

## 5. `dtwiz recommend`

- [ ] **Command runs**
  - [ ] `dtwiz recommend` exits with code 0 in a typical environment with no Dynatrace credentials required.

- [ ] **Uses or repeats analysis**
  - [ ] `dtwiz recommend` returns environment-based recommendations (not a static list).

- [ ] **OTel-first logic implementation**
  You can verify by manipulating environment state:

  - [ ] With an existing OTel Collector:
    - [ ] `dtwiz recommend` includes `install otel-update` as a recommendation.
  - [ ] Without OTel Collector:
    - [ ] `dtwiz recommend` includes `install otel-collector` as a primary recommendation.

  - [ ] With a Kubernetes cluster accessible:
    - [ ] `dtwiz recommend` includes `install kubernetes`.

  - [ ] With Docker but no Kubernetes:
    - [ ] `dtwiz recommend` includes `install docker`.

  - [ ] On a bare host (no Docker, no K8s; as far as detection can tell):
    - [ ] `dtwiz recommend` includes `install oneagent`.

  - [ ] Where OneAgent is already installed:
    - [ ] Recommendation output reflects that (e.g., informs no additional OneAgent install is needed).

  - [ ] With AWS credentials:
    - [ ] `dtwiz recommend` includes `install aws`.

  - [ ] With Azure credentials:
    - [ ] `dtwiz recommend` includes `install azure`.

  - [ ] With GCP credentials:
    - [ ] `dtwiz recommend` includes `install gcp`.

- [ ] **Explanations exist**
  - [ ] Each recommendation line includes some short explanation text (you can see a “because X detected” message or similar).

- [ ] **Optional `--json`**
  - [ ] If `dtwiz recommend --json` is implemented:
    - [ ] It outputs valid JSON.
  - [ ] If not implemented:
    - [ ] There is no `--json` flag for `recommend`.

---

## 6. `dtwiz setup`

- [ ] **Command exists and runs**
  - [ ] `dtwiz setup` is listed under `dtwiz --help`.
  - [ ] Running `dtwiz setup` in a simple test environment completes without a panic.

- [ ] **Automatic analysis + recommendation**
  - [ ] When you run `dtwiz setup`, it prints:
    - [ ] Detected environment details or a summary.
    - [ ] One or more recommended install options.

- [ ] **Interactive selection**
  - [ ] `dtwiz setup` allows you to:
    - [ ] Accept a default recommendation, or
    - [ ] Select another offered option (if multiple exist).

- [ ] **Performs an installation**
  In a test environment (e.g., dev cluster or local machine):
  - [ ] After finishing, the relevant installer command has clearly been executed (you can see created resources, processes, or logs).

- [ ] **`--dry-run`**
  - [ ] `dtwiz setup --dry-run` completes without making changes.
  - [ ] Output includes which installers it *would* call.

---

## 7. `dtwiz install ...` Subcommands

For each of the following, check:

1. The subcommand exists in `dtwiz install --help`.
2. It runs without panicking.
3. In a test environment, it performs the expected action or at least clearly documents that it is a stub/TODO.

### 7.1 `install otel-collector`

- [ ] `dtwiz install otel-collector` is listed under `install --help`.
- [ ] With valid environment URL and token:
  - [ ] It downloads or uses a Dynatrace OTel Collector binary.
  - [ ] It writes a config file with Dynatrace OTLP endpoint.
  - [ ] It starts the collector (service/process), or prints instructions if manual start is required.

- [ ] `--dry-run` (if implemented):
  - [ ] Does not create or modify files.
  - [ ] Prints what it would do.

### 7.2 `install otel-update`

- [ ] `dtwiz install otel-update` is listed.
- [ ] With a valid `--config` path pointing to an existing OTel config:
  - [ ] It modifies the config to add a Dynatrace exporter/endpoint.
  - [ ] It either:
    - [ ] Creates a backup of the original config, or
    - [ ] Prints the path where backup is stored.
- [ ] If OTel Collector is running:
  - [ ] It restarts it automatically or prints a clear “please restart” message.

### 7.3 `install kubernetes`

- [ ] `dtwiz install kubernetes` is listed.
- [ ] With `kubectl` configured to a test cluster:
  - [ ] The command applies or installs resources (Helm or manifests).
  - [ ] Dynatrace components (Operator/ActiveGate/OneAgent/Collector, depending on scope) appear in the cluster.

- [ ] `--dry-run` (if implemented):
  - [ ] Shows intended manifests/Helm actions without applying.

### 7.4 `install oneagent`

- [ ] `dtwiz install oneagent` is listed.
- [ ] On a test Linux or Windows host (where allowed):
  - [ ] The command downloads/uses the official OneAgent installer.
  - [ ] It starts the installer and completes without manual input when using `--quiet` (if supported).

- [ ] Flags:
  - [ ] `--dry-run` (if present) does not install anything.
  - [ ] `--host-group` sets a host group value in the configuration or installer parameters.

### 7.5 `install docker`

- [ ] `dtwiz install docker` is listed.
- [ ] With Docker running:
  - [ ] The command starts a Dynatrace OneAgent container.
  - [ ] Re-running the command does not cause obvious breakage (idempotent behavior or clear error).

- [ ] `--dry-run` (if implemented) does not run a container, only prints the docker run command.

### 7.6 `install aws`

- [ ] `dtwiz install aws` is listed.
- [ ] With AWS credentials configured and required permissions in a test account:
  - [ ] The command creates or updates a CloudFormation stack.
  - [ ] After completion, AWS monitoring entities are visible in Dynatrace (can be validated manually once).

- [ ] `--dry-run`:
  - [ ] Shows intended stack name and template reference without creating it.

### 7.7 `install azure`

- [ ] `dtwiz install azure` is listed.
- [ ] With Azure credentials and a test subscription:
  - [ ] The command registers or configures Dynatrace Azure monitoring as per prototype design.

- [ ] `--dry-run` shows what would be created/changed.

### 7.8 `install gcp`

- [ ] `dtwiz install gcp` is listed.
- [ ] With GCP credentials and a test project:
  - [ ] The command configures GCP monitoring integration (service account / metrics export, etc., depending on prototype state).

- [ ] `--dry-run` shows planned actions without changes.

---

## 8. `dtwiz uninstall ...`

Check that uninstall subcommands:

1. Exist in `dtwiz uninstall --help`.
2. Run without crashing.
3. Actually remove previously created resources (in a test environment).

- [ ] **`uninstall otel-collector`**
  - [ ] Stops the collector process or prints a clear “stop manually” instruction.
  - [ ] Removes or clearly identifies the installation directory.

- [ ] **`uninstall kubernetes`**
  - [ ] Removes the Dynatrace-related resources (via Helm uninstall or manifest delete).
  - [ ] Leaves the cluster in a clean state (no leftover Dynatrace namespaces/pods, unless documented).

- [ ] **`uninstall oneagent`**
  - [ ] Invokes the native OneAgent uninstaller or prints the exact uninstall command.

- [ ] **`uninstall aws`**
  - [ ] Deletes the CloudFormation stack created by `install aws`.

- [ ] **`uninstall self`**
  - [ ] Removes the `dtwiz` binary from the installation location.
  - [ ] If PATH entries are modified by the installer, it reverts or instructs the user on manual cleanup.

---

## 9. `dtwiz status`

- [ ] `dtwiz status` is listed under `--help`.
- [ ] With a configured environment and token:
  - [ ] The command checks connection to the Dynatrace environment.
  - [ ] It indicates whether the token is valid.
  - [ ] It lists any detected Dynatrace agents/collectors as “healthy” or “not detected”.

- [ ] Exit codes:
  - [ ] 0 when environment and token are OK.
  - [ ] Non-zero when connectivity or auth is broken.

---

## 10. Service User & Platform Token Handling

- [ ] **Missing platform token**
  - [ ] When a feature requiring platform token is used without `DT_PLATFORM_TOKEN` / `--platform-token`, the command:
    - [ ] Fails with a clear message explaining a platform token is needed.

- [ ] **Missing permissions**
  - [ ] When platform token / service user creation fails due to permissions (in a test tenant where you can reproduce this):
    - [ ] The error mentions insufficient permissions or IAM issues (not just a generic HTTP error).

- [ ] **No silent failures**
  - [ ] K8s and AWS installation commands do **not** silently “succeed” when required platform tokens/service users are missing:
    - [ ] They either fail clearly, or
    - [ ] Warn explicitly that the integration could not be fully configured.

---

## 11. Post-Install Validation

For any installer that is implemented (at least one path should be):

- [ ] **Process / resource checks**
  - [ ] After running the installer, the CLI:
    - [ ] Checks whether the relevant process (agent/collector) is running, or
    - [ ] Checks whether K8s resources are in a ready state.

- [ ] **Dynatrace-side checks (if implemented)**
  - [ ] The CLI calls a Dynatrace API or constructs a URL to verify data/entities.
  - [ ] If this is not yet implemented, there is at least a TODO or placeholder in code.

- [ ] **User-facing link**
  - [ ] The CLI prints a Dynatrace URL (e.g., hosts, K8s, logs, or services view) where the user can manually confirm incoming data.

---

## 12. UX & Output

- [ ] **Help text**
  - [ ] `dtwiz --help` and each subcommand’s `--help` output is present and understandable.
  - [ ] Flags and descriptions are non-empty and reasonably clear.

- [ ] **Exit codes**
  - [ ] Non-successful operations return non-zero exit codes (can be tested by a failing command).

- [ ] **Sensitive data**
  - [ ] Tokens and secrets are never printed back in logs or error messages.

- [ ] **Optional JSON output**
  - [ ] If any `--json` flag is present:
    - [ ] The output is valid JSON for success and error cases where applicable.

---
