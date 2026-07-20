# Design

## Context

The existing Azure integration used `dtwiz-azure` as a hardcoded name for all
created resources. Two problems arise from this:

1. Multiple Dynatrace environments pointing at the same Azure tenant each try to
   create an App Registration named `dtwiz-azure`. The second install finds the
   existing app and tries to patch it, which requires elevated Graph permissions
   most users do not have.

2. A partial install that failed midway leaves an orphaned App Registration. A
   retry finds it and hits the same patching error, even though the user has
   full ownership of the resource.

## Goals / Non-Goals

**Goals:**

- Give each Dynatrace environment its own stable, predictable resource name.
- Keep uninstall able to clean up resources from both the old and new naming.
- Make legacy cleanup non-fatal so it does not block current resource removal.

**Non-Goals:**

- Fix the underlying Azure permissions gap (that is a separate preflight check).
- Change the GCP integration naming.
- Add a user-facing flag to override the generated name.

## Decisions

- Use the first DNS label of the Dynatrace environment URL as the name suffix.
  - Rationale: it is visible in the URL the user already knows, stable across
    sessions, requires no extra CLI calls, and is unique per DT environment.
  - Alternative considered: Azure subscription ID prefix. Rejected because it would
    not distinguish two DT environments on the same subscription.
  - Alternative considered: signed-in Azure user identity. Rejected because it
    breaks for service principals in CI pipelines and ties the resource to a person
    rather than the integration being set up.

- Use prefix matching in Dynatrace resource discovery during uninstall.
  - Rationale: `dtwiz-azure` is a prefix of `dtwiz-azure-<tenant-id>`, so a single
    prefix query finds both old and new resources without needing to know which name
    was used at install time.

- Search Azure AD under both the current env-scoped name and the legacy fixed name
  during uninstall.
  - Rationale: orphaned App Registrations from old installs may still exist under
    `dtwiz-azure`. A single display-name search does exact matching, so two searches
    are needed.

- Treat legacy App Registration cleanup failures as warnings, not errors.
  - Rationale: the legacy resource may have been created by a different user who
    still owns it. The current user may lack permission to delete it. This must not
    prevent the current integration from being removed cleanly.

- Run account lookup before DT resource discovery in the update flow.
  - Rationale: the integration name is derived from the environment URL, not the
    account, so the order does not affect correctness. However, account lookup is
    a preflight gate: if the user is not logged in, there is no point querying DT.

## Risks / Trade-offs

- Two Azure AD searches in uninstall instead of one adds a small latency cost.
  Mitigation: both are read-only and run sequentially only during uninstall.
- Prefix matching in DT resource discovery could theoretically match unrelated
  connections if someone created a connection starting with `dtwiz-azure` manually.
  Mitigation: the federated credential fingerprint check already gates which Azure
  App Registrations are safe to delete.
