# Tasks

## 1. Watch Data Model

- [x] 1.1 Add a Hosts section to the watch state in `pkg/installer/ingest_watch.go`.
- [x] 1.2 Query regular host and OpenTelemetry host Smartscape entities with host IDs, names, and entity types.
- [x] 1.3 Parse host rows into a combined host count and detail links.

## 2. Rendering

- [x] 2.1 Render sections in the order Services, Hosts, Kubernetes, Cloud, Relationships, Logs, Requests, Exceptions.
- [x] 2.2 Link regular hosts and OpenTelemetry hosts to their correct InfraOps detail pages.
- [x] 2.3 Keep host details compact by limiting visible hosts and showing a `+N more` suffix.

## 3. Tests

- [x] 3.1 Add parser tests for regular hosts, OpenTelemetry hosts, mixed host lists, missing fields, and truncation.
- [x] 3.2 Add rendering/query coverage for host links and section order.
- [x] 3.3 Run focused Go tests for the installer watch package.
