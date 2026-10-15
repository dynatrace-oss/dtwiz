## Context

RUM onboarding has three phases: detect whether the tag can be injected (`otel-rum-detection`), create the frontend and fetch its tag, then inject the tag or print it for manual insertion. This change covers only the printing half of phase 3, as a library function nothing calls yet. It does not depend on detection being implemented, so the two changes can be built in parallel.

Step 2 will save only the frontend ID locally and fetch the tag every time it is needed, so the printer always receives a freshly fetched string. Until step 2 exists, the tag is a fixed dummy value in the tests.

Content follows the [agentless monitoring guide](https://docs.dynatrace.com/docs/observe/digital-experience/rum/web-frontends/initial-setup/set-up-agentless-monitoring): add the tag to the `<head>` of every page as the first executable script, use a shared template when there is one, and expect data in Experience Vitals within about ten minutes. The recommended format for manual insertion is the "JavaScript tag", a single `<script src="https://js-cdn.dynatrace.com/jstag/..." crossorigin="anonymous"></script>` line that updates itself.

## Goals / Non-Goals

**Goals:**

- Render the tag and manual insertion instructions to an `io.Writer`.
- Keep the tag line copyable: no indentation, color, or borders.
- Optional reason and framework hint lines, with content supplied by the caller.
- Golden-output unit tests.

**Non-Goals:**

- Calling the function from `dtwiz install otel`, the install preview, or a dry run. A follow-on change wires it in; a dry run must never print a tag, because getting a real tag means creating a frontend.
- Fetching the tag or creating a frontend.
- A hint in the `WatchIngest` footer. The follow-on wiring change shows it always in manual mode, and it refers to the tag printed above instead of repeating it, because a wrapped tag would break the footer's cursor-up redraw.
- A frontend signal in `WatchIngest`. A frontend entity exists before any traffic, so a useful signal needs a DQL query over real RUM events. It would also bring the snapshot User-Agent close to its 64-character limit.
- Telemetry. `install.rum` stays `not_tried`. Later, `f=` keeps its three values and a body-only `install.rum_mode` property carries `auto` or `manual`.
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

Three positional strings are easy to swap by accident; named fields are not. The tag is a plain string rather than an interface or provider: the caller fetches it, and a test constant is enough until step 2 exists.

The function returns an error for an empty tag (after trimming whitespace) and for write errors. Instructions without a tag are useless, and an empty tag line would look like a broken install, so nothing is printed in that case.

### Writer, not stdout

The function writes to `w` so the tests can compare output byte for byte. The `pkg/display` helpers write to stdout, so the function formats lines itself with `fmt.Fprintf` and uses `display.ColorHeader` / `display.ColorMuted` with `Fprintf` for styled parts. The caller passes `os.Stdout` (or `color.Output`) later.

### Output layout

```
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

`pkg/installer/otel/rum/manual.go` with tests in `manual_test.go`. RUM is OTel-specific, so it sits under the OTel method folder (folder-per-method rule). The detection change adds `detect.go` to the same package; keeping the two concerns in separate files follows the file-per-concern rule.

### Test patch for reviewers

Because nothing calls the function, the PR description contains a short patch that calls `PrintManualInstructions(os.Stdout, ...)` with the dummy tag (for example from the end of `InstallOtelCollectorWithProject`), so reviewers can see the real output in a terminal. The patch is not committed.

## Risks / Trade-offs

- [The dummy tag looks real and could be pasted into an app] → It exists only in tests and in the uncommitted test patch; no command can reach the function.
- [Instructions drift from the docs] → They are kept to the few stable points in the guide, and the RUM documentation is linked for everything else.
- [A very long tag wraps in narrow terminals] → Wrapping is visual only; the copied text is still one line, because the line contains nothing but the tag.
- [Unused exported code until the follow-on change lands] → Accepted; golangci-lint does not flag unused exported functions, and the tests exercise the code.

## Migration Plan

None. The code is unreachable from the CLI; rollback is deleting the file.

## Open Questions

- None for this change. The exact framework hint texts and the wiring point (end of `InstallOtelCollectorWithProject`, before `WatchIngest`) are decided in the follow-on change.
