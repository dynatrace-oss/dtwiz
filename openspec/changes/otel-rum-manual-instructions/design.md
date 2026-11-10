# Design: OTel RUM Manual Instructions

## Context

RUM onboarding has three phases: detect whether the tag can be injected (`otel-rum-detection`), create the frontend and fetch its tag, then inject the tag or print it for manual insertion. This change covers only the printing half of phase 3, as a library function nothing calls yet. It does not depend on detection being implemented, so the two changes can be built in parallel.

Phase 2 is split across two changes:

- Tag retrieval is implemented (`rum-javascript-tag`): `rum.FetchJavaScriptTag` in `pkg/installer/otel/rum/tag.go` returns the OpenTelemetry JavaScript tag for a known frontend ID, with surrounding whitespace trimmed.
- Frontend creation is planned (`create-and-store-frontend-id`). It stores only the frontend ID in `.dtwiz/config.yaml`, so the tag is fetched every time it is needed and the printer always receives a fresh string.

The tag format returned by `GET /platform/rum/v1/manualInsertionTags/javaScriptTag/{frontendId}` is not documented in the repo. The printer treats the tag as an opaque string and does not depend on its shape.

Content follows the [agentless monitoring guide](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/set-up-agentless-monitoring): add the tag to the `<head>` of every page as the first executable script, use a shared template when there is one, and expect data in Experience Vitals within about ten minutes.

## Goals / Non-Goals

**Goals:**

- Render the tag and manual insertion instructions to an `io.Writer`.
- Keep the tag line copyable: no indentation, color, or borders.
- Optional reason and framework hint lines, with content supplied by the caller.
- Golden-output unit tests.

**Non-Goals:**

- Calling the function from `dtwiz install otel`, the install preview, or a dry run. A follow-on change wires it in; a dry run must never print a tag, because getting a real tag means creating a frontend.
- Calling `rum.FetchJavaScriptTag` or creating a frontend. The caller fetches the tag and passes it in.
- A hint in the `WatchIngest` footer. The follow-on wiring change shows it always in manual mode, and it refers to the tag printed above instead of repeating it, because a wrapped tag would break the footer's cursor-up redraw.
- A frontend signal in `WatchIngest`. A frontend entity exists before any traffic, so a useful signal needs a DQL query over real RUM events. It would also bring the snapshot User-Agent close to its 64-character limit.
- Telemetry. `install.rum` stays `not_tried`. The recording rule is set by `create-and-store-frontend-id`: `FeatureRUM` is recorded as failed on any RUM failure and as succeeded only after the whole RUM setup finishes. Printing the tag for manual insertion counts as finishing the setup. `f=` keeps its three values; a body-only `install.rum_mode` property carrying `auto` or `manual` is added by the wiring change.
- Framework hint texts (for example Next.js or Nuxt placement). They belong to the caller that knows the detected framework.
- CSP, CORS, and devtools troubleshooting. The linked RUM documentation leads to them.

## Decisions

### API: a struct parameter

```go
type ManualInstructions struct {
    Tag    string // required
    Reason string // optional: why automatic injection is not possible
    Hint   string // optional: where to put the tag in the detected framework
}

func PrintManualInstructions(w io.Writer, in ManualInstructions) error
```

Three positional strings are easy to swap by accident; named fields are not. The tag is a plain string rather than an interface or provider: the caller fetches it (via `rum.FetchJavaScriptTag`), and a test constant is enough for this change.

The function returns an error for an empty tag (after trimming whitespace) and for write errors. `FetchJavaScriptTag` already trims the tag; trimming again is cheap and keeps the printer correct for any caller. Instructions without a tag are useless, and an empty tag line would look like a broken install, so nothing is printed in that case.

### Writer, not stdout

The function writes to `w` so the tests can compare output byte for byte. The `pkg/display` helpers write to stdout, so the function formats lines itself with `fmt.Fprintf` and uses `display.ColorHeader` / `display.ColorMuted` with `Fprintf` for styled parts. The caller passes `os.Stdout` (or `color.Output`) later.

### Output layout

```text
  ── Real User Monitoring ──
  Automatic injection isn't possible: <Reason>.
  Add this tag to your app manually:

<Tag>

  Where:  in <head> of every page, as the first script
          (use your shared layout or template if you have one)
  Hint:   <Hint>
  Verify: data appears in Experience Vitals within ~10 min
  Docs:   <docs URL>
```

- The heading matches the existing `── <Runtime> auto-instrumentation ──` style in `otel.go`.
- The "Automatic injection" line is left out when `Reason` is empty, and the "Hint" line when `Hint` is empty. The other lines are unchanged.
- The tag is flush left between blank lines, so a triple-click selects exactly the tag. A box (`display.PrintInfoBox`) was rejected because its borders would be copied along with the tag.
- The tag is never colored, even when the rest of the output is.
- The labels (`Where`, `Hint`, `Verify`, `Docs`) are padded to the same width so the values line up.

### Docs URL as a constant

```go
const rumDocsURL = "https://docs.dynatrace.com/docs/observe/digital-experience/rum"
```

The link points to the RUM documentation overview rather than the agentless setup guide the instructions are based on. The overview is a stable entry point to the setup, snippet format, and troubleshooting pages.

No `dt-url.net` short link exists yet. When one does, a single constant changes. The URL is printed as plain text, not as an OSC 8 hyperlink, so it stays visible when the output is copied or logged.

### File placement

`pkg/installer/otel/rum/manual.go` with tests in `manual_test.go`, in the existing `rum` package next to `tag.go`. RUM is OTel-specific, so it sits under the OTel method folder (folder-per-method rule). Tag retrieval (`tag.go`), detection (`detect.go`, planned) and the printer are separate concerns, so each gets its own file.

`create-and-store-frontend-id` currently plans frontend creation in a separate package, `pkg/installer/otel/internal/rum/`. That would split RUM across two packages named `rum`. This change does not depend on the outcome, but the placement should be settled with that change before either is implemented.

### Dummy tag in tests

The test constant follows the shape already used in `tag_test.go`: `<script src="https://example.invalid/rum.js"></script>`. The `.invalid` domain makes it clearly fake, and it claims no format the real endpoint has not been checked against.

### Test patch for reviewers

Because nothing calls the function, the PR description contains a short patch that calls `PrintManualInstructions(os.Stdout, ...)` at the end of `InstallOtelCollectorWithProject`, so reviewers can see the output in a terminal. By default the patch uses the dummy tag. A reviewer who has created a frontend by hand can set its ID in an environment variable, and the patch then fetches the real tag with `rum.FetchJavaScriptTag` and prints that instead. The patch is not committed.

## Risks / Trade-offs

- [The real tag is longer or has a different shape than the dummy] → The printer treats the tag as an opaque string. The optional real-tag path in the test patch shows the actual output during review.
- [Instructions drift from the docs] → They are kept to the few stable points in the guide, and the RUM documentation is linked for everything else.
- [A very long tag wraps in narrow terminals] → Wrapping is visual only; the copied text is still one line, because the line contains nothing but the tag.
- [Unused exported code until the follow-on change lands] → Accepted; golangci-lint does not flag unused exported functions, and the tests exercise the code.

## Migration Plan

None. The code is unreachable from the CLI; rollback is deleting `manual.go` and `manual_test.go`.

## Open Questions

- None for this change. The exact framework hint texts and the wiring point (end of `InstallOtelCollectorWithProject`, before `WatchIngest`) are decided in the follow-on change.
