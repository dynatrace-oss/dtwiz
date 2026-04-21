# Tasks: Ingest Watch

## 1. Dependencies

- [ ] 1.1 Add `golang.org/x/term` to `go.mod` via `go get golang.org/x/term`

## 2. Core Implementation (`pkg/installer/ingest_watch.go`)

- [ ] 2.1 Define data structs: `watchData` (per-section counts/details), `watchState` (all sections + start time)
- [ ] 2.2 Implement DQL query executor reusing `dqlResponse` pattern from `pkg/installer/otel_env.go`
- [ ] 2.3 Implement query functions for all 7 sections (services, cloud, kubernetes, relationships, logs, requests, exceptions)
- [ ] 2.4 Implement type name humanization (strip prefix, lowercase, pluralize)
- [ ] 2.5 Implement terminal renderer with ANSI cursor-up for in-place updates
- [ ] 2.6 Implement non-TTY fallback (append-only output)
- [ ] 2.7 Implement `WatchIngest(envURL, token string)` main entry point with 5-second poll loop
- [ ] 2.8 Implement elapsed time formatting (Xm Ys)
- [ ] 2.9 Implement deep link generation using `AppsURL()` for all sections + QuickStart

## 3. Cobra Command (`cmd/watch.go`)

- [ ] 3.1 Create `dtwiz watch` command with `cobra.NoArgs`, wired to `WatchIngest()`
- [ ] 3.2 Resolve environment URL and platform token from flags/env vars using existing `cmd/auth.go` helpers

## 4. Installer Integration

- [ ] 4.1 Add `WatchIngest()` call to each installer's success path (oneagent, kubernetes, docker, otel, aws)

## 5. Testing

- [ ] 5.1 Run `make test` — all existing tests pass
- [ ] 5.2 Run `make lint` — no new lint issues
- [ ] 5.3 Manual verification: `make build && ./dtwiz watch` against a live Dynatrace environment
