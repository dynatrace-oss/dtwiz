# Design

## Context

Both integrations used a hardcoded resource name (`dtwiz-gcp`, `dtwiz-azure`) for
all Dynatrace and cloud resources created during install. This caused collisions when
two Dynatrace environments pointed at the same cloud account, and when a failed install
left orphaned resources that a retry would hit again.

## Goals / Non-Goals

**Goals:**
- Give each Dynatrace environment its own stable, predictable resource name.
- Keep uninstall able to clean up resources from both old and new naming schemes.
- Make legacy cleanup non-fatal.

**Non-Goals:**
- Add a user-facing flag to override the generated name.
- Fix underlying cloud permission gaps (separate concern).

## Decisions

### Naming scheme

Use the first DNS label of the Dynatrace environment URL as the name suffix
(`dtwiz-gcp-fds1499d`, `dtwiz-azure-fds1499d`). This identifier is already visible
in the URL the user knows, is stable across sessions, requires no extra CLI calls,
and is unique per DT environment.

Alternatives considered: cloud-specific IDs (subscription ID, project ID) — rejected
because they don't distinguish two DT environments on the same account; signed-in
identity — rejected because it breaks for service accounts and CI pipelines.

**GCP constraint**: GCP service account IDs must be 6–30 characters. `dtwiz-gcp-`
is 10 characters; typical 8-character tenant IDs give 18 total, well within the limit.

### Uninstall discovery

Use prefix matching in Dynatrace resource discovery so a single query covers both old
and new names.

**Azure only**: two Azure AD display-name searches are needed (one for the env-scoped
name, one for the legacy fixed name) because display-name search is exact-match only.
App Registrations found by display-name lookup are included only if they carry the
dtwiz federated credential fingerprint.

**GCP only**: the legacy service account email is derived deterministically from
`integrationPrefix` and the GCP project ID — no extra CLI lookup needed.

### Legacy cleanup

Treat legacy resource cleanup failures as warn-only. The legacy resource may belong
to a different DT environment or user; the current uninstall must not be blocked by
something it doesn't own.

### Azure update order

Run account lookup before DT resource discovery in the update flow. Account lookup
is a preflight gate — if the user is not logged in there is no point querying DT.

## Risks / Trade-offs

- Prefix matching in DT resource discovery could theoretically match resources
  manually created with a `dtwiz-gcp` or `dtwiz-azure` prefix. Risk is low; for
  GCP an extra SA email check gates actual cleanup, and for Azure the federated
  credential fingerprint check gates App Registration deletion.
