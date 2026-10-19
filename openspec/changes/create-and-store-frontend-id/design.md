# Design: Create and Store Frontend ID

## Context

dtwiz has no mechanism for persisting project-local state between runs. All existing installers are stateless: they compute what to do from the environment each time. Agentless RUM breaks this pattern because the Dynatrace RUM API creates a new frontend object on every POST, and the resulting ID must survive across invocations so subsequent steps can retrieve the frontend-specific JS snippet.

The RUM frontend creation endpoint (`POST /platform/rum/v1/frontends`) lives in the Platform URL family (`.apps.`) and is authenticated with a Bearer platform token — the same pattern as existing Platform API calls in `pkg/client/`.

This change introduces two new packages. Neither touches existing code; they are consumed by a follow-on change that wires them into the otel installers.

## Goals / Non-Goals

**Goals:**

- Provide a general-purpose project config file (`pkg/config/`) usable by any future feature, not just RUM.
- Provide an idempotent `EnsureFrontendApplication` function in `pkg/installer/otel/internal/rum/` that creates a `WEB_AGENTLESS` frontend application and caches its ID locally.
- Keep the config format human-readable and stable across dtwiz versions.

**Non-Goals:**

- Wiring `EnsureFrontendApplication` into `install otel` or `update otel` (follow-on change).
- Any CLI command or flag surface for RUM.
- Config migration tooling (format is new; no existing files to migrate).
- Deleting or updating a frontend on the tenant.

## Decisions

### Decision: Project config file at `{project-dir}/.dtwiz/config.yaml`

The file lives in the instrumented project's directory so each project owns its own config independently. It is keyed by environment URL inside the file so the same project can be used against multiple Dynatrace tenants without conflict.

YAML was chosen over JSON for readability: the file is small, potentially human-inspected, and `gopkg.in/yaml.v3` is already a dependency.

File name `.dtwiz/config.yaml` (inside a `.dtwiz/` subdirectory) namespaces dtwiz tooling metadata clearly without polluting the project root.

**Alternative considered:** `{project-dir}/.dtwiz.yaml` (flat file). Rejected in favour of the directory form to leave room for future per-project config without accumulating dot-files in the project root.

### Decision: Config keyed by environment URL (not tenant ID)

The environment URL is the value the user already provides via `DT_ENVIRONMENT` and is what dtwiz uses everywhere else for identification. Using it as the map key means no extra extraction logic and the config is self-explanatory when opened in an editor.

**Alternative considered:** keying by tenant ID (first DNS label). More compact, but requires an extra extraction step and is less readable.

### Decision: API reference and required token scopes

The frontend creation call follows the Dynatrace Platform RUM API reference: [POST /platform/rum/v1/frontends](https://docs.dynatrace.com/docs/dynatrace-api/platform-api/platform-rum-v1/frontends-post). Checked against that page: path and method match; `displayName`, `enableRum` and `type` are required; `type` accepts `CORDOVA`, `MOBILE`, `WEB_AGENTLESS` and `WEB_AUTO_INJECTED` (we send `WEB_AGENTLESS`); `frontendName` is listed as optional, must be unique across web and mobile, and cannot be changed later (we always send it, so the name is deterministic); the 201 body returns `id`, `displayName`, `frontendName` and `type`. The page documents 400, 4XX and 5XX with a common error envelope (`error.code`, `error.message`, optional `error.constraintViolations`) but does not list 401, 403 or 409 separately.

Required platform token scopes: `rum:frontends:write` for this call, and `rum:manual-insertion-tags:read` for retrieving the JS snippet in the follow-on step. Both are already listed in the README's token-scope table. On a 401 or 403 the error names `rum:frontends:write` as the likely fix, phrased as a starting point rather than a certain diagnosis, following the precedent of the OpenPipeline error enrichment (a missing scope and a restrictive policy look the same in the response).

### Decision: Request shape verified against a tenant

A manual `POST /platform/rum/v1/frontends` with the body `{displayName, frontendName, type: "WEB_AGENTLESS", enableRum: true}` and a Bearer platform token was accepted with HTTP 201 and returned an `id`. Posting the same `frontendName` again returned HTTP 400 with `{"error":{"code":400,"message":"The frontend name must be unique across the environment."}}`. The request body, the endpoint path and the duplicate behavior used in this design are based on that check.

### Decision: RUM failure is a warning, not an install failure

RUM is an additional feature on top of the OTel install. If provisioning the frontend fails for any reason, the caller warns, skips the remaining RUM steps, records `installer.FeatureRUM` as failed (so the `ist` self-monitoring event carries `install.rum=failed` and `0` in the RUM position of `f=`), and lets the rest of the install continue. `EnsureFrontendApplication` itself only returns the error; the warn, skip and record behavior is the caller's job and is implemented in the follow-on wiring change. `FeatureRUM` is recorded as succeeded only after the whole RUM setup finishes, because the last recorded value for a feature wins.

### Decision: `frontendName` generated as `dtwiz-{sanitized-dir}-{sha256[:8] of abs path}`

`frontendName` is immutable on the tenant and must be globally unique across web and mobile frontends. Generating it deterministically from the absolute project path ensures:

- The same project directory always produces the same name (idempotent across runs and machines with the same checkout path).
- The `dtwiz-` prefix makes the origin obvious in the Dynatrace UI.
- The 8-char path hash disambiguates projects that share a directory name.

`displayName` is set to `{dir-name} [dtwiz]` for human readability in the UI.

**Alternative considered:** random suffix (UUID). Rejected because it would produce a new `frontendName` on each run if the config is missing, causing repeated frontend creation on the tenant.

### Decision: No version field in config (for now)

The config format does not include a version field. YAML's permissive unmarshalling makes additive changes (new optional fields) automatically backward compatible, so a version field is only needed for destructive changes (key renames, structural reorganisation). Those are not anticipated for this simple structure.

An absent version field can be treated as `version: 1` at any future point, so the field can be introduced when a migration is actually needed without losing existing files. If a near-term breaking change becomes known, this decision should be revisited and a version field added before the first release.

### Decision: `pkg/config/` as a standalone package

The config read/write logic is a distinct concern (persistent project-local state) shared across current and future features. Placing it in `pkg/installer/` root would stretch that package's responsibility. A dedicated `pkg/config/` package matches the single-responsibility principle and is reachable from any other package without creating import cycles.

### Decision: RUM frontend logic lives in `pkg/installer/otel/internal/rum/`

Agentless RUM is not a standalone install method — it is provisioned automatically as part of the otel install and update flows. There is no `dtwiz install rum` command. Adding it as a top-level `pkg/installer/rum/` package would violate the folder-per-method rule.

Placing it inside `pkg/installer/otel/internal/rum/` gives it a proper package boundary (avoiding type-name collisions and keeping the otel package from growing unbounded), while the `internal/` restriction enforces that nothing outside the otel subtree can import it. If the scope ever grows to require sharing across methods, moving it out of `internal` is a deliberate, visible decision.

### Decision: `EnsureFrontendApplication` signature takes a `*client.PlatformClient` and project directory

The caller (future otel installer) already holds a `*client.Client` from `setupClient()`. Accepting `*client.PlatformClient` directly avoids re-constructing credentials and stays consistent with how other installers receive their client.

The project directory is accepted as an explicit parameter rather than inferred from `os.Getwd()` so the function is testable without process-level side effects.

## Risks / Trade-offs

**Risk: Two projects with the same directory name on different machines produce the same `frontendName` if their absolute paths match.** Unlikely in practice (absolute paths almost always differ) and the path hash provides strong disambiguation. Accepted.

**Risk: The config file may be accidentally committed with a staging environment's frontend ID.** The file is not secret (IDs are not credentials), and the per-environment keying means a staging ID in the file does not break production. We intentionally do not touch `.gitignore` — committing the file is valid and even useful for team-shared projects. Accepted.

**Risk: Orphaned frontends and name conflicts.** The frontend is created on the tenant before the ID is saved locally, and `frontendName` is deterministic per absolute path. If `Save` fails after a successful POST, or the project config file is deleted, the next run POSTs the same `frontendName`, which the tenant rejects as a duplicate (verified: HTTP 400 with `The frontend name must be unique across the environment.`). A moved or renamed project directory gets a different path hash, so it creates a new frontend and orphans the old one instead of failing.

We have no verified list or get-by-name API for frontends, so we do not attempt automatic recovery. Instead, that rejection is turned into a dedicated error that names the `frontendName`, links straight to the tenant's frontends list (`{apps-url}/ui/apps/dynatrace.experience.vitals/frontends/` appended to `installer.AppsURL(envURL)`), and offers two ways out from there: copy the existing frontend's ID into `.dtwiz/config.yaml` under the current environment URL, or [delete the frontend](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/additional-configuration/delete-frontend-web) and re-run. The `frontendName` is printed so the user can find the entry in the list. Accepted: recovery is manual, but the failure is clear and non-destructive. Because the API uses plain 400 for this, detection matches status 400 plus a message containing "unique" (case-insensitive). This is brittle against message rewording; the fallback is the generic error, which still includes the API's message, so a rewording degrades the guidance but never hides the failure. If a lookup API is confirmed later, adopting the existing ID on conflict can replace this.

## Open Questions

None.
