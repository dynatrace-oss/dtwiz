# Design

## Context

The existing GCP integration used `dtwiz-gcp` as a hardcoded name for all created
resources. Two problems arise from this:

1. Multiple Dynatrace environments pointing at the same GCP project each try to
   create a service account named `dtwiz-gcp`. The second install creates a duplicate
   SA or the uninstaller for one environment deletes the SA owned by another.

2. A partial install that failed midway leaves an orphaned service account. A retry
   reuses it (the `already exists` path is tolerated), but uninstall from a different
   environment would also attempt to delete it.

## Goals / Non-Goals

**Goals:**

- Give each Dynatrace environment its own stable, predictable resource name.
- Keep uninstall able to clean up resources from both the old and new naming.
- Make legacy cleanup non-fatal so it does not block current resource removal.

**Non-Goals:**

- Change the Azure integration naming.
- Add a user-facing flag to override the generated name.

## Decisions

- Use the first DNS label of the Dynatrace environment URL as the name suffix.
  - Rationale: it is visible in the URL the user already knows, stable across
    sessions, requires no extra CLI calls, and is unique per DT environment.
  - Alternative considered: GCP project ID suffix. Rejected because it would not
    distinguish two DT environments on the same project.
  - Alternative considered: signed-in GCP user identity. Rejected because it breaks
    for service accounts in CI pipelines and ties the resource to a person rather
    than the integration being set up.

- GCP service account IDs must be 6-30 characters of lowercase letters, digits, and
  hyphens. `dtwiz-gcp-` is 10 characters; DT tenant IDs are typically 8 lowercase
  alphanumeric characters, giving a total of 18 characters well within the limit.

- Use prefix matching in Dynatrace resource discovery during uninstall.
  - Rationale: `dtwiz-gcp` is a prefix of `dtwiz-gcp-<tenant-id>`, so a single
    prefix query finds both old and new resources without needing to know which name
    was used at install time.

- Derive the legacy service account email deterministically from `integrationPrefix`
  and the GCP project ID. No CLI lookup is needed.
  - Rationale: GCP SA discovery does not require a display-name search (unlike Azure
    App Registrations). The email is fully predictable from the old fixed name.

- Treat legacy service account cleanup failures as warnings, not errors.
  - Rationale: the legacy SA may have been created by a different install against a
    different DT environment. That environment's uninstall is responsible for it. This
    must not prevent the current integration from being removed cleanly.

## Risks / Trade-offs

- Prefix matching in DT resource discovery could theoretically match unrelated
  connections if someone created a connection starting with `dtwiz-gcp` manually.
  This risk is low; such a connection would have to carry a matching service account
  email to be acted on during SA cleanup.
- Legacy SA cleanup adds two extra steps to uninstall when a legacy SA exists. Both
  are gcloud calls and fail gracefully (not-found is success in `gcpDeleteServiceAccount`
  and `gcpRemoveProjectBinding`).
