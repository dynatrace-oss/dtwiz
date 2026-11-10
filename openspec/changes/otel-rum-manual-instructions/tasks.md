# Tasks: OTel RUM Manual Instructions

## 1. Package and types

- [ ] 1.1 Add `pkg/installer/otel/rum/manual.go` to the existing `rum` package (next to `tag.go`).
- [ ] 1.2 Add the `rumDocsURL` constant with the RUM documentation URL (`https://docs.dynatrace.com/docs/observe/digital-experience/rum`).
- [ ] 1.3 Define the `ManualInstructions` struct with `Tag`, `Reason`, and `Hint` string fields and doc comments marking `Reason` and `Hint` optional.

## 2. Rendering

- [ ] 2.1 Implement `PrintManualInstructions(w io.Writer, in ManualInstructions) error`: trim whitespace from `Tag`; if it is empty, return an error without writing anything.
- [ ] 2.2 Write the `── Real User Monitoring ──` heading using `display.ColorHeader.Fprintf(w, ...)`.
- [ ] 2.3 Write the `Automatic injection isn't possible: <Reason>.` line only when `Reason` is not empty, then the "Add this tag to your app manually:" line.
- [ ] 2.4 Write the tag flush left, uncolored, between two blank lines.
- [ ] 2.5 Write the `Where`, `Hint` (only when not empty), `Verify`, and `Docs` lines with labels padded to the same width; print the docs URL as plain text.
- [ ] 2.6 Return the first write error, wrapped with context.

## 3. Tests

- [ ] 3.1 Create `pkg/installer/otel/rum/manual_test.go` with a dummy tag constant in the shape used by `tag_test.go` (`<script src="https://example.invalid/rum.js"></script>`), and disable colors (`color.NoColor = true`, restored with `t.Cleanup`) for golden comparisons.
- [ ] 3.2 Table-driven golden-output test into a `bytes.Buffer` with cases: tag only; tag + reason; tag + hint; tag + reason + hint.
- [ ] 3.3 Test that a tag with surrounding spaces and newlines is printed trimmed, on a line containing nothing else.
- [ ] 3.4 Test that empty and whitespace-only tags return an error and leave the buffer empty.
- [ ] 3.5 Test with colors enabled (`color.NoColor = false`) that the tag line contains no `\x1b` escape sequences.
- [ ] 3.6 Test that a failing writer makes the function return an error.
- [ ] 3.7 Run `go test ./pkg/installer/otel/rum/...` and `make lint`; both must pass.

## 4. Review aid

- [ ] 4.1 Prepare the uncommitted test patch for the PR description: call `rum.PrintManualInstructions(os.Stdout, ...)` with a sample reason and a sample hint at the end of `InstallOtelCollectorWithProject` in `pkg/installer/otel/otel.go`, plus the command to run it.
- [ ] 4.2 In the patch, use the dummy tag by default; when an environment variable holds a frontend ID (from a frontend created by hand), fetch the real tag with `rum.FetchJavaScriptTag(ctx, envURL, platformToken, id)` and print that instead.
