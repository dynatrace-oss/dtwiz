## Context

`dtwiz setup` prints two blocks back to back: the System Analysis produced by `analyzer.SystemInfo.Summary()`, then the recommendation list. Together they are the only information a first-time user has when choosing an ingestion method, and the moderated testing round showed both blocks failing at that job.

Constraints shaping this design:

- **80 columns.** The menu must stay readable in a default terminal. Any per-entry text describing which signals are ingested pushes the longest entry past 80 and wraps only that line, which reads as a rendering defect.
- **Two renderers, already drifted.** `setup` renders the list inline while `recommend` uses `recommender.FormatRecommendations`. They differ in divider width (50 vs 42), badge style (`[1]` vs ` 1 `), trailer options, and feature-flag filtering. Any wording added has to be added in both places.
- **`Title` is a public field.** It is serialized by `recommend --json`, so retitling is visible to anything parsing that output.
- **Display-only.** The findings are about comprehension, not behavior. Nothing here should alter detection, ranking, or dispatch.

## Goals / Non-Goals

**Goals:**

- Make monitoring scope the visible difference between the recommendation entries, so the choice can be made from the menu alone.
- State once, and accurately, which signals every option ingests.
- Remove the two labels in the System Analysis block that name something other than what they carry.
- Place the two lines that determine the menu contents where they are read before the menu.

**Non-Goals:**

- Unifying the `setup` and `recommend` renderers.
- Fixing `recommend` ignoring the `Experimental` filter, which makes it advertise an entry `setup` will not offer.
- Removing the unused `Priority`, `ComingSoon`, and `MethodNotSupported` fields, or the unreachable render branches that read them.
- Changing which recommendations are produced, their order, or their gating.
- Surfacing OneAgent's host-wide instrumentation scope at install time — that is the other half of the same finding and belongs with the installer's preview.

## Decisions

**Hoist the signal list into a lead-in rather than per entry.**
A single `Monitor Logs, Metrics, Traces of:` above the list names the signals once and turns each entry into a grammatical object completing it. Per-entry alternatives were tried first and all failed the 80-column constraint: naming signals inside a title costs roughly 40 characters, which pushes the longest entry to 117. A two-line-per-entry layout fits but triples the list's height and repeats a clause identical across every entry. The lead-in costs one line total and is the only option that scales as recommendations are added.

**Differentiate on scope, keeping a shared noun phrase.**
Every entry keeps `This host and its services` and varies only in the parenthetical. Making the entries fully distinct was considered and rejected: all three genuinely monitor the same host, and only trace instrumentation differs, so wording that implies different footprints would be inaccurate. Naming the mechanism in the parenthetical — existing collector, new collector, OneAgent — is the honest distinction and reads as one decision along one axis.

**`This host` over `This machine`.**
Dynatrace's own entity vocabulary is Host, and dtwiz already says host in `install oneagent`'s help text and the `--host-group` flag. Choosing "machine" would open a vocabulary seam at the handoff into the Dynatrace UI, which is exactly where the tested users lost the thread. "Host" risks reading as *a server somewhere* to a laptop developer; the System Analysis block resolves that by printing the actual hostname on the `This host:` line directly above.

**`Runtimes` over `Services`.**
`detectServices()` resolves runtime binaries with `which` and probes a few daemons with `pgrep`. "Services" overstates that — it implies running workloads, and it collides with "services" in the recommendation titles directly below, where the word means something else. The line keeps its position at the bottom of the block: it is the only line in the analysis that no recommendation reads, so it is the least decision-relevant and belongs last.

**Move OneAgent up rather than reordering the whole block.**
OneAgent and OpenTelemetry are the only lines answering "is this host already monitored?", and the only two whose state changes the menu below — `OneAgentRunning` suppresses the OneAgent entry, `OtelCollector` adds the existing-collector entry. Grouping them under `This host:` puts the causes adjacent and directly above their effect. OpenTelemetry is already in position 2 and the runtimes line is already last, so this reduces to moving one block up five positions — a smaller diff than a full reshuffle, and it leaves the container and cloud lines in their established order.

## Risks / Trade-offs

**The retitled `Title` values appear in `recommend --json`.** → Anything parsing that output for exact title strings breaks. Mitigated by treating `Method` as the stable identifier — it is unchanged, and it is what `setup` already dispatches on. Called out under Changed in the changelog.

**Three entries still share the same six opening words.** → Under `--experimental` with a collector running, the difference is one word inside the parentheses. Accepted: they describe genuinely similar scopes, and the parentheticals now carry a real distinction rather than restating the noun phrase. The larger clarity gap for this finding is at install time, where OneAgent's host-wide scope is still not previewed at all.

**The lead-in is duplicated across two renderers.** → It must be edited in both `cmd/setup.go` and `FormatRecommendations` or they drift again. Accepted deliberately: unifying the renderers is a refactor with a behavior fix attached and does not belong in a wording change.

**`recommend` advertises an entry `setup` will not offer.** → Pre-existing and untouched here; the new lead-in makes the list look more authoritative without making it more consistent. Tracked as follow-up.
