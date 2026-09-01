## Context

`dtwiz watch` polls Dynatrace DQL and renders a fixed set of watch sections from `pkg/installer/ingest_watch.go`. The current Smartscape node aggregation excludes host identity data, so a fresh OneAgent install can leave the watch display looking empty even though the host entity exists in Dynatrace.

## Goals / Non-Goals

**Goals:**

- Show regular host and OpenTelemetry host entities together in a new `Hosts` section.
- Provide a clickable detail link for each displayed host.
- Preserve compact watch output by capping the visible host list and using `+N more` for overflow.
- Render sections in recommender order: Services, Hosts, Kubernetes, Cloud, Relationships, Logs, Requests, Exceptions.

**Non-Goals:**

- Add a separate OTel hosts section.
- Change install behavior or host monitoring setup.
- Add new CLI flags or feature gates.

## Decisions

- Query host identity separately from the existing node type aggregation.
  - Rationale: the current node query groups by type and cannot provide names or IDs needed for per-host detail links.
  - Alternative considered: derive hosts from the all-node aggregation. This would only provide counts and would not satisfy host detail links.
- Represent host rows as formatted detail links inside the existing watch section model.
  - Rationale: this keeps the rendering path small and preserves the compact watch layout.
  - Alternative considered: create a dedicated renderer just for hosts. That would add more branching for a section that otherwise behaves like the existing count-plus-details sections.
- Route detail links by entity type.
  - Rationale: regular hosts and OTel hosts use different InfraOps Smartscape routes, but both belong under the same `Hosts` section.

## Risks / Trade-offs

- Host Smartscape data may lag behind installation -> the `Hosts` section continues to show `waiting...` until host entities are returned.
- DQL field names for host identifiers may differ by entity type -> tests cover the expected `id`, `name`, and `type` parsing, and unknown or incomplete rows are ignored.
- Long host names can make output noisy -> only the first five hosts are displayed, followed by `+N more`.