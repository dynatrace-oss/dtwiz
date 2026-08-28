# Tasks

## 1. Display package fix

- [x] 1.1 Add `visibleWidth()` helper in `pkg/display/print.go` that strips ANSI color and OSC 8 hyperlink escape sequences before measuring rune length
- [x] 1.2 Update `PrintInfoBox`'s padding calculation to use `visibleWidth()` instead of raw `len([]rune(line))`
- [x] 1.3 Add/verify unit tests in `pkg/display/print_test.go` covering plain-text, hyperlink, and blank-separator rows (`TestPrintInfoBox_RendersContentRow`, `TestPrintInfoBox_HyperlinkLine_BorderAligned`, `TestPrintInfoBox_BlankLineRendersEmptyRow`)

## 2. OTel install notice

- [x] 2.1 Add `printSupportedRuntimesInfoBox()` in `pkg/installer/otel/otel.go`, using `display.PrintInfoBox` and the existing `display.Hyperlink`/`display.StdoutSupportsHyperlinks()` pattern for the walkthroughs link
- [x] 2.2 Call `printSupportedRuntimesInfoBox()` once per install run inside the project-selection retry loop, after the detected-projects list and before `selectProject()`'s prompt, guarded so retries don't reprint it

## 3. Verification

- [x] 3.1 `go build ./...`, `go vet ./...`, `gofmt -l` clean
- [x] 3.2 `go test ./pkg/display/... ./pkg/installer/otel/...` passes (excluding pre-existing, unrelated Java-environment test failures on this machine)
- [x] 3.3 Manually verified box rendering with a simulated OSC 8 hyperlink line to confirm border alignment
