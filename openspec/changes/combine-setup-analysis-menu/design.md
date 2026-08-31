# Design: Combine System Analysis and Recommendation Menu

## Context

`dtwiz setup` runs in three phases: analyze → recommend → install. Currently it prints
a full System Analysis block (`info.Summary()`) between the analysis and recommendation
phases. The recommendation menu that follows re-expresses most of the same data (what
cloud providers are connected, what infrastructure was found). Users must read both blocks
to understand their situation before picking an option.

The analyzer package already holds all detection data in `SystemInfo`. The recommender
already receives `SystemInfo` and uses it to decide which recommendations to emit. The
missing piece is surfacing detection context at the point of decision — the menu — rather
than as a separate preceding block.

## Goals / Non-Goals

**Goals:**

- Surface detection context inline on each menu entry so users read one block, not two.
- Show undetected cloud/infra options as greyed, non-selectable entries with an unlock
  command, so users know what monitoring they can add without leaving the tool.
- Detect the project tech stack in the current working directory and attach it to the
  host recommendation, giving users immediate confirmation that their code is in scope.
- Keep `dtwiz analyze` and `dtwiz status` unchanged — they still own the full
  System Analysis block.
- Keep `pkg/analyzer` as the single source of detection truth; no detection logic moves
  to the recommender or cmd layer.

**Non-Goals:**

- Subdirectory scanning for project detection (cwd only).
- Reusing the OTel installer's per-runtime project discovery; that logic is
  installation-specific and richer than what the menu needs.
- Changing the selection, validation, or dispatch logic in `dtwiz setup`.

## Decisions

### Detection context lives on `Recommendation`, not in the renderer

**Decision:** Add `DetectionInfo string`, `Unavailable bool`, `ShortTitle string`, and
`UnlockCommand string` fields to `Recommendation`. The recommender populates them;
the renderer reads typed fields.

**Alternatives considered:**

- Parse detection context from `SystemInfo` inside `cmd/setup.go` — rejected because it
  puts analyzer-layer knowledge into the cmd layer and creates an implicit contract
  between two packages.
- Format detection info as a string inside the renderer — rejected because it requires
  parsing strings that the recommender already owns as structured data, repeating the
  same smell that motivated the `ShortTitle`/`UnlockCommand` split.

### Unavailable recommendations are always emitted by `GenerateRecommendations`

**Decision:** `GenerateRecommendations` appends `Unavailable=true` entries for
Kubernetes, AWS, Azure, and GCP whenever those are not detected. The renderer decides
whether and how to show them; `FormatRecommendations` (used by `dtwiz recommend`) skips
them.

**Alternatives considered:**

- Compute unavailable entries only inside `FormatSetupMenu` — rejected because it
  mixes recommendation logic with rendering logic.
- Expose a separate `GenerateUnavailable(system)` function — rejected as unnecessary
  indirection; unavailable entries are part of the full recommendation picture.

### Menu rendering moves to the recommender package

**Decision:** `FormatSetupMenu(recs, demoRunning, experimental)` is added to
`pkg/recommender`. `cmd/setup.go` calls it as a single statement. `ActionableItems()`
is also added to the recommender so the cmd layer does not duplicate the filtering logic.

**Alternatives considered:**

- Keep rendering inline in `cmd/setup.go` — the rendering loop is short enough that
  inline is defensible, but moving it to the recommender keeps `setup.go` as thin
  orchestration and makes the rendering testable without invoking a Cobra command.

### Project detection runs concurrently with other detectors

**Decision:** `detectProject()` is added to `AnalyzeSystem()`'s concurrent fan-out
alongside the existing detectors. It returns the shortened cwd path and a `[]ProjectTech`
slice; both are stored on `SystemInfo`.

**Rationale:** File-stat calls on a handful of fixed filenames are fast but not instant.
Running them concurrently avoids adding latency to the setup flow.

## Risks / Trade-offs

- **cwd is not always the project root** → `detectProject` only scans the immediate cwd,
  not parents. Users running `dtwiz setup` from a subdirectory will see no techs.
  Mitigation: this is informational only; the menu remains fully functional regardless.
- **Unavailable entries inflate the recommendation slice** → downstream code that
  iterates `recs` without filtering on `Unavailable` may process unexpected entries.
  Mitigation: `FormatRecommendations` already skips them; `ActionableItems` excludes them
  by definition. Callers that need all entries can check the field.
- **`system-analysis-output` spec covers both `analyze` and `setup`** → removing the
  block from setup requires a delta spec update so the requirement no longer includes
  `dtwiz setup` as a surface.
