## Context

After a successful `dtwiz install`, users have no immediate feedback that data is flowing into Dynatrace. The existing `waitForServices()` in `otel_env.go` polls for service names but is limited to OTel installs and doesn't cover logs, requests, cloud, or Kubernetes resources. Users must manually navigate multiple Dynatrace apps to verify ingestion.

The codebase already has DQL query infrastructure (`dqlResponse` struct, HTTP POST to the Platform query API) and terminal coloring via `fatih/color` with magenta as the highlight color.

## Goals / Non-Goals

**Goals:**
- Provide a live-updating terminal summary of all newly ingested data categories after install
- Work as both a standalone `dtwiz watch` command and auto-triggered post-install
- Show direct deep links to relevant Dynatrace apps for each data category
- Gracefully degrade to non-live output when stdout is not a TTY

**Non-Goals:**
- Historical data analysis or trending — only shows data from the watch start time
- Replacing Dynatrace UI — this is a quick verification tool, not a monitoring dashboard
- Supporting custom DQL queries — the data sections are fixed

## Decisions

### 1. ANSI cursor movement for in-place rendering (vs. TUI library)
Use `\033[<N>A` (cursor up) to overwrite previous output each poll cycle. This avoids adding a TUI dependency (like bubbletea) and keeps the output simple and pipe-friendly. Fall back to append-only mode if `golang.org/x/term.IsTerminal()` returns false.

**Alternative**: bubbletea TUI framework — rejected because it's a heavy dependency for a simple status display and doesn't align with the project's minimal dependency approach.

### 2. Single file for core logic (`ingest_watch.go`)
All polling, query execution, data aggregation, and rendering live in one file in `pkg/installer/`. This follows the existing pattern where each feature has a self-contained file (e.g., `otel_env.go`, `otel_collector.go`).

### 3. Reuse existing DQL infrastructure
Use the same `dqlResponse` struct and HTTP POST pattern from `otel_env.go`. The `AppsURL()` + `/platform/storage/query/v1/query:execute` endpoint with Bearer auth.

### 4. Poll interval of 5 seconds
Balances responsiveness with API load. Matches the existing `waitForServices()` pattern which uses 5-second intervals.

### 5. Type name humanization for Cloud/K8s
Strip prefix (`AWS_`, `K8S_`), replace `_` with space, lowercase, pluralize. Simple string manipulation — no lookup table needed.

### 6. `golang.org/x/term` for TTY detection
Already an indirect dependency via `golang.org/x/sys`. Lightweight, stdlib-adjacent. Only used for `term.IsTerminal(int(os.Stdout.Fd()))`.

## Risks / Trade-offs

- **[DQL API rate limiting]** → Mitigation: 5-second poll interval keeps request rate low (~1.2 req/s across 6 queries). Queries use short timeouts (10s) and small result limits.
- **[Platform token required]** → Mitigation: Watch gracefully skips if no platform token is available, printing a message instead of failing.
- **[Terminal width assumptions]** → Mitigation: Output is designed to fit 80 columns. Long service names are truncated with `+N more` pattern.
- **[Ctrl+C handling]** → The watch loop runs until interrupted. No special signal handling needed beyond Go's default behavior.
