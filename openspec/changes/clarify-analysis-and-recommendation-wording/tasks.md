## 1. System Analysis block

- [x] 1.1 Rename the operating-system label in `Summary()` from `Platform` to `This host` in `pkg/analyzer/analyzer.go`
- [x] 1.2 Move the OneAgent rendering block in `Summary()` so it follows the OpenTelemetry block and precedes the Docker block
- [x] 1.3 Rename the detected-runtimes label from `Services` to `Runtimes`, keeping the line last in the block
- [x] 1.4 Confirm the label width constant still aligns every line after the renames

## 2. Recommendation titles

- [x] 2.1 Retitle the existing-collector recommendation to `This host and its services (via existing OpenTelemetry Collector)` in `pkg/recommender/recommender.go`
- [x] 2.2 Retitle the new-collector recommendation to `This host and its services (via new OpenTelemetry Collector)`
- [x] 2.3 Retitle the OneAgent recommendation to `This host and its services (via OneAgent)`

## 3. Signal lead-in

- [x] 3.1 Print `Monitor Logs, Metrics, Traces of:` followed by a blank line after the recommendations header in `cmd/setup.go`
- [x] 3.2 Print the same lead-in after the divider in `FormatRecommendations` in `pkg/recommender/recommender.go` so `recommend` matches `setup`

## 4. Tests

- [x] 4.1 Update the System Analysis assertions in `pkg/analyzer/analyzer_test.go` from `Services:` to `Runtimes:`
- [x] 4.2 Add a `This host:` assertion to `pkg/analyzer/analyzer_test.go` so the renamed label is covered
- [x] 4.3 Run `go test ./pkg/analyzer/... ./pkg/recommender/... ./cmd/...` and confirm all pass
- [x] 4.4 Verify `analyze --json` still emits the original `platform` key

## 5. Verification and documentation

- [x] 5.1 Render `dtwiz analyze`, `dtwiz status`, `dtwiz setup`, `dtwiz setup --experimental`, and `dtwiz recommend` from a fresh build and confirm each matches the specs
- [x] 5.2 Confirm the longest recommendation entry stays within 80 columns
- [x] 5.3 Add a `Changed` entry to `CHANGELOG.md` covering the renamed labels, the reordered line, the lead-in, and the retitled recommendations
