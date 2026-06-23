# Task: Implement the Azure connection installer in dtwiz (`dtwiz install azure`)

Replace the stub in `pkg/installer/azure.go` with a real implementation that creates a
Dynatrace↔Azure connection using **federated identity credentials**, driving Dynatrace-side
operations through the **dtctl** CLI and Azure-side operations through the **az** CLI. Mirror
the structure, style, and UX of the existing AWS installer.

## Reference material (read first)

- `pkg/installer/aws.go` — THE template to follow. Match its structure: a config struct,
  prerequisite checks, a compact preview of every command before execution, a single
  `Apply? [Y/n]` confirmation, dry-run support, and masked tokens in logs.
- `pkg/installer/aws_uninstall.go` — mirror for an `UninstallAzure` later (out of scope unless trivial).
- `cmd/install.go` around the existing `installAzureCmd` (calls `installer.InstallAzure()`),
  and how `installAWSCmd` wires credentials: `getDtEnvironment()` → `validateCredentials()` →
  pass env URL + platform token into the installer. Update the Azure command to match the AWS
  signature (pass env URL, platform token, dry-run flag, start time).
- `pkg/installer/installer.go` — reuse `RunCommand`, `classicAPIURL`/`AppsURL`, token masking helpers.
- `pkg/analyzer/detect_azure.go` / `analyzer.go` `AzureInfo` — already detects `SubscriptionID`
  and `TenantID` via `az account show`. Reuse these rather than re-shelling where possible.
- `pkg/display` — for headers, status lines, colored output. Keep output minimal (see AGENTS.md
  "Reduce to the max").
- Official flow (authoritative for exact flags):
  https://docs.dynatrace.com/docs/ingest-from/microsoft-azure-services/create-an-azure-connection/azure-connection-cli#create-a-new-azure-connection-cli

## Decisions already made (do not re-ask)

1. **dtctl missing → error fast with an install hint.** Add an `isDtctlInstalled()` check
   (`exec.LookPath("dtctl")`) mirroring `isAWSCLIInstalled()`. Also check `az` is present.
   On missing dtctl, print a clear message pointing to the dtctl install docs and return an error.

2. **dtctl auth = reuse dtwiz's own credentials via env vars, do NOT mutate the user's dtctl
   config.** FIRST investigate how dtctl accepts ambient auth: check `dtctl --help`,
   `dtctl config --help`, and whether dtctl honors `DT_ENVIRONMENT` / `DT_PLATFORM_TOKEN`
   environment variables or has equivalent global flags (e.g. `--environment` / `--token`).
   Prefer passing dtwiz's already-resolved env URL and platform token to each dtctl invocation
   by setting `DT_ENVIRONMENT` and `DT_PLATFORM_TOKEN` in the command's environment (via
   `exec.Cmd.Env`), or via flags if those exist. Only fall back to a temporary
   `dtctl config set-context`/`set-credentials` if no ambient mechanism exists — and if you must,
   use an isolated/throwaway context name and document the tradeoff. Verify your assumption about
   dtctl's env-var support before committing to it; if you cannot verify, leave a clearly marked
   TODO and implement the flag/env path you believe is correct.

3. **Monitoring scope = Management Group.** The Monitoring Reader role assignment must target
   `/providers/Microsoft.Management/managementGroups/<MANAGEMENT_GROUP_ID>`, not a subscription.
   Auto-detect the management group: try `az account management-group list` and select the
   tenant root group (id typically equals the tenant ID) when present; if exactly one group
   exists use it; if multiple non-root groups exist, fall back to the tenant root group as the
   broadest scope. Keep user interaction to a minimum — only prompt if detection is genuinely
   ambiguous and no root group is found.

4. **Zero-config feature/region filtering: all on.** Call `dtctl create azure monitoring` WITHOUT
   `--locationFiltering` or `--featureSets` so Dynatrace defaults apply broadly. (Matches AWS
   "all services, all regions" zero-config principle.)

## Preflight (only the two authoritative gates; run BEFORE any mutation)

Do NOT preflight Dynatrace token scopes, and do NOT preflight tenant setup (Fleet Management /
Invex / license / phase) — those are out of scope. Do NOT hard-gate on Entra directory rights
(not reliably checkable; handled as an attempt failure instead). Implement exactly these two:

- **CLI presence + active sessions (hard gate):**
  `exec.LookPath("dtctl")` and `exec.LookPath("az")`; confirm an active Azure session via
  `az account show` (the analyzer already does this) and optionally `az account get-access-token`
  to catch an expired session before mutating. Abort with a clear, per-tool hint if any fails.
- **Azure RBAC role-assignment write at the chosen mgmt-group scope (hard gate):**
  Use the Resource Manager checkAccess API, e.g.
  `az rest --method POST --url "https://management.azure.com/providers/Microsoft.Management/managementGroups/<MG>/providers/Microsoft.Authorization/checkAccess?api-version=2022-04-01" --body '{"actions":[{"id":"Microsoft.Authorization/roleAssignments/write"}]}'`
  Abort before creating anything if access is not "Allowed". (This has no analog in the Dynatrace
  Clouds app — it exists because dtwiz performs the role assignment itself rather than delegating
  it to a wizard.)

## The flow to implement (federated identity)

Resolve a CONNECTION_NAME and CONFIGURATION_NAME (derive a sane default, e.g. include the
subscription/tenant or a fixed `dtwiz-azure` prefix; do not force the user to type one).

1. **Create empty connection (dtctl):**
   `dtctl create azure connection --name <CONNECTION_NAME> --type federatedIdentityCredential`
   → Parse and capture the returned **connection object ID**. Prefer JSON output if dtctl
   supports an output flag (investigate `--output`/`-o`), else parse table output robustly.

2. **Register Azure service principal (az):**
   `az ad sp create-for-rbac --name <CONNECTION_NAME> --create-password false --query "{CLIENT_ID:appId, TENANT_ID:tenant}" -o json`
   → capture CLIENT_ID (appId) and TENANT_ID. Use `-o json` and unmarshal, don't parse tables.

3. **Create federated credential (az):**
   `az ad app federated-credential create --id <CLIENT_ID> --parameters <json>`
   where the JSON is:
   `{"name":"<CONNECTION_NAME>-Federated-Credential","issuer":"https://token.dynatrace.com","subject":"dt:connection-id/<CONNECTION_ID>","audiences":["<DYNATRACE_ENVIRONMENT_ID>.apps.dynatrace.com/svc-id/com.dynatrace.da"]}`
   - Build this JSON with `encoding/json` (do not hand-concatenate). The audience uses the
     ENVIRONMENT_ID (the first DNS label of the apps URL — reuse `ExtractTenantID`) and the
     `.apps.dynatrace.com` form; derive it from dtwiz's env URL via the existing AppsURL/host
     helpers and handle the `dynatracelabs.com` variant too.

4. **Get service principal object ID (az):**
   `az ad sp show --id <CLIENT_ID> --query "{OBJECT_ID:id}" -o json` → capture OBJECT_ID.

5. **Assign Monitoring Reader at management-group scope (az):**
   `az role assignment create --assignee-object-id <OBJECT_ID> --role "Monitoring Reader" --scope "/providers/Microsoft.Management/managementGroups/<MGMT_GROUP_ID>" --assignee-principal-type ServicePrincipal --description "Dynatrace Monitoring"`

6. **Update connection with SP details (dtctl):**
   `dtctl update azure connection --name <CONNECTION_NAME> --directoryId <TENANT_ID> --applicationId <CLIENT_ID>`

7. **Create monitoring configuration (dtctl):**
   `dtctl create azure monitoring --name <CONFIGURATION_NAME> --credentials <CONNECTION_NAME>`
   (no filtering flags — zero-config).

## Cross-cutting requirements

- **Function signature:** `func InstallAzure(envURL, platformToken string, dryRun bool, startTime time.Time) error`
  (match AWS; adjust call site in `cmd/install.go`). Keep the no-arg stub gone.
- **Preview + confirm:** before executing, print each command (one line, tokens masked) and a
  single `Apply? [Y/n]` (default yes). Honor the global `--yes/-y` and `--dry-run` flags exactly
  like AWS. In dry-run, print everything and execute nothing.
- **Cancellation:** return `ErrInstallCancelled` (the sentinel AWS uses) when the user declines.
- **State carried between steps:** CONNECTION_ID (step 1) → step 3 subject; CLIENT_ID/TENANT_ID
  (step 2) → steps 3/4/6; OBJECT_ID (step 4) → step 5. Use a small config/state struct like
  `awsStackConfig`.
- **Eventual consistency:** after `az ad sp create-for-rbac`, the new app/SP may not be
  immediately queryable. Wrap `az ad sp show`, `az ad app federated-credential create`, and
  `az role assignment create` in bounded retry-with-backoff that distinguishes "not found /
  propagation" from a real authorization failure.
- **Idempotency / partial-failure:** wrap each step's error with context
  (`fmt.Errorf("...: %w")` including the failing command). On failure, print which artifacts were
  already created (SP, federated credential, role assignment, DT connection) so the user can
  clean up. Bonus: detect "already exists" errors from az/dtctl and treat them as non-fatal where safe.
- **Token masking:** never print the platform token; reuse the AWS `maskTokenArgs`-style helper.
- **Cross-platform:** dtctl and az are external CLIs — use `exec.LookPath` and `RunCommand`; no
  Unix-only assumptions. Use `context.Context` propagation if RunCommand supports it.
- **Logging:** use `logger.Debug()` for each command invocation and parsed IDs (mask secrets).

## Open items to resolve during implementation (investigate, don't guess silently)

- Exact dtctl global auth mechanism (env vars vs flags) — verify before wiring decision #2.
- dtctl output format flags for reliably parsing the connection object ID.
- `az account management-group list` output shape and how to identify the tenant root group;
  handle the case where the Management Groups feature is not enabled on the tenant.

## Testing requirements (mandatory)

Do not shell out to real `az`/`dtctl` in any automated test. Inject the command runner.

1. **Make exec injectable.** Refactor so command execution goes through an interface/func field
   (e.g. `type cmdRunner func(ctx, name string, args ...string) (stdout string, err error)`),
   defaulting to the real `RunCommand`. Tests supply a fake runner that returns canned
   stdout/err per (name, args) pattern.

2. **Pure-function unit tests (table-driven, like existing *_test.go):**
   - connection-object-ID parsing from dtctl output (JSON and table variants, malformed input)
   - federated-credential JSON construction — assert exact issuer/subject/audiences, including
     `dynatrace.com` vs `dynatracelabs.com`, and env-ID extraction from the apps URL
   - management-group selection (tenant root present; single group; multiple non-root → fallback
     or prompt; MG feature disabled → graceful error)
   - argument builders for every az/dtctl command (correct flags, scope string format)
   - token/secret masking in previewed and logged commands (assert the token never appears)

3. **Preflight tests with the fake runner:**
   - missing dtctl / missing az → abort with the right per-tool hint; zero mutations
   - `az account show` failure (not logged in / expired) → abort; zero mutations
   - RBAC checkAccess returns "NotAllowed" → abort BEFORE any mutation; assert nothing created
   - RBAC checkAccess "Allowed" → proceeds into the flow

4. **Flow tests with the fake runner (no real CLIs):**
   - happy path: assert the exact ordered sequence of commands and that state flows correctly
     (CONNECTION_ID → subject; CLIENT_ID/TENANT_ID → later steps; OBJECT_ID → role assignment)
   - dry-run: assert ZERO mutating commands run and the preview contains every command
   - `--yes`: assert no prompt is read
   - confirmation declined: assert ErrInstallCancelled and zero mutations

5. **Failure-injection tests (one per failing step):**
   - dtctl `create azure connection` fails → abort with wrapped, command-specific error;
     assert no az mutations attempted afterward
   - `az ad sp create-for-rbac` fails (e.g. Entra denial) → abort; assert later steps not run
   - `az role assignment create` fails → assert the error reports which artifacts were already
     created (SP, federated credential, DT connection)
   - eventual-consistency: `az ad sp show` returns "not found" twice then succeeds → assert the
     bounded retry succeeds and the transient case is not surfaced as a fatal error

6. **Golden/snapshot test** for the preview block (the "following will run" output) so UX/output
   changes are reviewed deliberately. Mask secrets in the golden file.

7. **Optional live e2e (guarded, not run in CI by default):** behind a build tag or
   `DTWIZ_LIVE_AZURE_TEST=1` env guard, plus a documented manual test checklist in the PR.

Coverage target: all pure functions and the flow orchestration (preflight, ordering, error
wrapping, the retry path) must be covered by table-driven tests with the fake runner.
`make build`, `make test`, and `make lint` must pass. No `//nolint` without justification.

Deliver: the rewritten `pkg/installer/azure.go` (split into helpers as AWS does), the updated
`cmd/install.go` wiring, and tests. Keep the diff focused; don't touch the GCP stub.
