# Tasks: Combine System Analysis and Recommendation Menu

## 1. Analyzer — Project Tech Detection

- [ ] 1.1 Add `ProjectTech` struct (`Name`, `Path` string fields with json tags) to `pkg/analyzer/analyzer.go`
- [ ] 1.2 Add `ProjectTechs []ProjectTech` and `ProjectDir string` fields to `SystemInfo` in `pkg/analyzer/analyzer.go`
- [ ] 1.3 Create `pkg/analyzer/detect_project.go` with `detectProject()` returning `(dir string, techs []ProjectTech)`
- [ ] 1.4 Implement tech indicator scanning in `detectProject()` covering Node.js, Go, Python, Java, Rust, Ruby, PHP, .NET patterns
- [ ] 1.5 Implement `shortenPath()` helper that replaces the home-directory prefix with `~`
- [ ] 1.6 Wire `detectProject()` into the concurrent fan-out in `AnalyzeSystem()` in `pkg/analyzer/analyzer.go`

## 2. Recommender — Detection Context and Unavailable Entries

- [ ] 2.1 Add `DetectionInfo`, `ShortTitle`, `UnlockCommand string` and `Unavailable bool` fields to `Recommendation` struct in `pkg/recommender/recommender.go`
- [ ] 2.2 Add `platformName(p analyzer.Platform) string` helper returning human-readable OS name
- [ ] 2.3 Add `hostDetectionInfo(system *analyzer.SystemInfo) string` helper that builds the detection context line (hostname · OS arch · tech stack)
- [ ] 2.4 Update `GenerateRecommendations` to populate `DetectionInfo` on each detected recommendation
- [ ] 2.5 Update `GenerateRecommendations` to always append `Unavailable=true` entries for Kubernetes, AWS, Azure, GCP when not detected, each carrying `ShortTitle` and `UnlockCommand`
- [ ] 2.6 Update `FormatRecommendations` to skip `Unavailable=true` entries (used by `dtwiz recommend`)
- [ ] 2.7 Add `ActionableItems(recs []Recommendation, experimental bool) []Recommendation` to `pkg/recommender/recommender.go`
- [ ] 2.8 Add `recFaint` color variable (`color.New(color.Faint)`) to `pkg/recommender/recommender.go`
- [ ] 2.9 Add `FormatSetupMenu(recs []Recommendation, demoRunning bool, experimental bool) string` to `pkg/recommender/recommender.go`
- [ ] 2.10 Render Done entries with `✓` badge and muted detection-context line in `FormatSetupMenu`
- [ ] 2.11 Render numbered actionable entries with muted detection-context line below each title in `FormatSetupMenu`
- [ ] 2.12 Render "Sign in to unlock:" section with greyed unavailable entries showing unlock commands in `FormatSetupMenu`

## 3. Setup Command — Remove Duplicate Analysis Block

- [ ] 3.1 Remove `fmt.Println(info.Summary())` call from `cmd/setup.go`
- [ ] 3.2 Replace inline recommendation rendering in `cmd/setup.go` with calls to `recommender.ActionableItems()` and `recommender.FormatSetupMenu()`

## 4. Tests

- [ ] 4.1 Add unit tests for `detectProject()` covering Go, Node.js, Python, Java indicator files and empty-directory case
- [ ] 4.2 Add unit tests for `shortenPath()` covering home-prefix replacement and paths outside home
- [ ] 4.3 Add unit tests for `hostDetectionInfo()` covering with/without project techs
- [ ] 4.4 Add unit tests for `ActionableItems()` verifying Done and Unavailable entries are excluded
- [ ] 4.5 Add unit tests for `FormatSetupMenu()` verifying numbered entries, Done entries, and unavailable section render correctly
- [ ] 4.6 Verify `FormatRecommendations` (used by `dtwiz recommend`) skips Unavailable entries
