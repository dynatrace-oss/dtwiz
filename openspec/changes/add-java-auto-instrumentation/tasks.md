# Tasks

## 1. Java Version Validation

**Files:** `pkg/installer/otel_java_process.go` (create), `pkg/installer/otel_java_process_test.go` (create)

- [x] 1.1 Create `pkg/installer/otel_java_process.go` with `parseJavaVersion(output string) (int, error)` — extract the quoted version string from `java -version` stderr; handle legacy (`1.8.0_382` → 8) and modern (`17.0.1` → 17, `21` → 21) formats
- [x] 1.2 Add `validateJavaPrerequisites() (string, error)` — check `java` in PATH via `exec.LookPath`, run `java -version`, parse the output, return error if version < 8. Return the java binary path on success.
- [x] 1.2a Add debug logging in `validateJavaPrerequisites` and `parseJavaVersion`:
  - Java binary found: `logger.Debug("java binary found", "path", javaPath)`
  - Version string parsed: `logger.Debug("java version parsed", "raw", rawOutput, "major", major)`
  - Version OK: `logger.Debug("java version OK", "major", major)`
  - Version too old: `logger.Debug("java version too old", "major", major, "minimum", 8)`
- [x] 1.3 Tests in `pkg/installer/otel_java_process_test.go`:
  - [x] `TestParseJavaVersion_Legacy_1_8` (input: `openjdk version "1.8.0_382"` → 8)
  - [x] `TestParseJavaVersion_Modern_17` (input: `java version "17.0.1" 2021-10-19` → 17)
  - [x] `TestParseJavaVersion_Short_21` (input: `openjdk version "21" 2023-09-19` → 21)
  - [x] `TestParseJavaVersion_OpenJDK_11` (input: `openjdk version "11.0.20" 2023-07-18` → 11)
  - [x] `TestParseJavaVersion_Unrecognized` (input: `not a valid version` → error)
  - [x] `TestParseJavaVersion_Java7_TooOld` (input: `java version "1.7.0_80"` → 7, then validate rejects it)

## 2. Agent JAR Download

**Files:** `pkg/installer/otel_java.go` (modify)

- [x] 2.1 Implement `downloadJavaAgent() (string, error)` — download the JAR from `otelJavaAgentURL` to `~/.opentelemetry/java/opentelemetry-javaagent.jar`. Create the directory if it does not exist. Use `net/http.Get` + `os.Create` + `io.Copy`. Return the absolute path to the JAR.
- [x] 2.2 Handle download errors: non-200 HTTP status → return error with URL and status code; network errors → return error with URL and error message.
- [x] 2.3 Output download progress via `display.PrintStatusLine("download", "OpenTelemetry Java agent... done.", display.ColorOK)`
- [x] 2.4 Tests in `pkg/installer/otel_java_test.go`:
  - [x] `TestDownloadJavaAgent_CreatesDirectory` — use a temp dir as destination; verify the `~/.opentelemetry/java/` directory is created when it does not exist, and the JAR file is written
  - [x] `TestDownloadJavaAgent_ErrorOnNon200` — mock an HTTP server returning 404; verify the function returns an error containing the URL and the HTTP status code
  - [x] `TestDownloadJavaAgent_NetworkError` — mock a server that closes the connection immediately; verify the function returns an error containing the URL

## 3. Java Entrypoint Detection

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

- [x] 3.1 Add `detectJavaEntrypoints(projectPath string) []JavaEntrypoint` — scan for runnable artifacts in the project directory. A `JavaEntrypoint` has `Command string` (the full launch command) and `Description string` (shown in the selection menu).
  - Scan `target/*.jar` and `build/libs/*.jar` for JARs with a `Main-Class` in `MANIFEST.MF` (use `archive/zip` to read the JAR).
  - If no fat JAR is found, resolve the best available build tool via `resolveMavenCmd`/`resolveGradleCmd`. Each resolver checks in order: (1) local wrapper script + bootstrap JAR (`mvnw` + `.mvn/wrapper/maven-wrapper.jar`, `gradlew` + `gradle/wrapper/gradle-wrapper.jar`); (2) system binary in PATH (`mvn`, `gradle`). A wrapper script without its bootstrap JAR is silently skipped and the system binary is tried instead. The resolved command prefix is used for both entrypoint detection and auto-build:
    - **Maven Spring Boot:** `<mvnCmd> spring-boot:run`
    - **Maven generic:** `<mvnCmd> exec:java`
    - **Gradle Spring Boot:** `<gradleCmd> bootRun`
    - **Gradle generic:** `<gradleCmd> run`
  - Add `isSpringBootMaven(projectPath string) bool` — reads `pom.xml` and checks for `spring-boot` substring.
  - Add `isSpringBootGradle(projectPath string) bool` — reads `build.gradle`/`build.gradle.kts` and checks for `spring-boot` or `springframework.boot` substrings.
- [x] 3.2 Add `isExecutableJar(jarPath string) bool` — open the JAR as a ZIP, read `META-INF/MANIFEST.MF`, return true if `Main-Class:` is present.
- [x] 3.2a Add debug logging throughout `detectJavaEntrypoints`:
  - When `target/` or `build/libs/` does not exist: `logger.Debug("dir not found, skipping JAR scan", "dir", path)`
  - When a JAR is accepted: `logger.Debug("executable JAR found", "jar", jarPath)`
  - When a JAR is rejected (no `Main-Class`): `logger.Debug("skipping JAR — no Main-Class in MANIFEST.MF", "jar", jarPath)`
  - After `isSpringBootMaven`/`isSpringBootGradle` evaluation: `logger.Debug("Spring Boot detection", "file", filePath, "result", result)`
  - When a wrapper fallback is chosen: `logger.Debug("no fat JAR found, using wrapper fallback", "command", cmd)`
  - When the result slice is empty: `logger.Debug("no entrypoint found", "project", projectPath, "scanned", scannedList)`
  - When exactly one candidate is auto-selected: `logger.Debug("auto-selected single entrypoint", "command", cmd)`
- [x] 3.2b Add debug logging for auto-build in `attemptSingleModuleBuild`:
  - Before running the build: `logger.Debug("attempting auto-build", "command", buildCmd, "project", projectPath)`
  - On success: `logger.Debug("auto-build succeeded", "project", projectPath)`
  - On failure: `logger.Debug("auto-build failed", "project", projectPath, "error", err)`
- [x] 3.3 Add `promptEntrypointSelection(entrypoints []JavaEntrypoint) *JavaEntrypoint` — when exactly one entrypoint is found, auto-select it (print the selection, no prompt); when multiple are found, present a numbered menu; return nil if user skips.
- [x] 3.4 Tests in `pkg/installer/otel_java_process_test.go`:
  - [x] `TestDetectJavaEntrypoints_MavenFatJar` (temp dir with `target/app.jar` containing `Main-Class` → returns jar candidate)
  - [x] `TestDetectJavaEntrypoints_GradleFatJar` (temp dir with `build/libs/app-all.jar` → returns jar candidate)
  - [x] `TestDetectJavaEntrypoints_MavenWrapperSpringBoot` (temp dir with `mvnw` + Spring Boot `pom.xml` → returns `spring-boot:run` candidate)
  - [x] `TestDetectJavaEntrypoints_MavenWrapperNonSpringBoot` (temp dir with `mvnw` + plain `pom.xml` → returns `exec:java` candidate)
  - [x] `TestDetectJavaEntrypoints_GradleWrapperSpringBoot` (temp dir with `gradlew` + Spring Boot `build.gradle` → returns `bootRun` candidate)
  - [x] `TestDetectJavaEntrypoints_GradleWrapperNoJar` (temp dir with `gradlew`, no Spring Boot → returns no entrypoint, falls through to auto-build)
  - [x] `TestDetectJavaEntrypoints_NoEntrypoint` (empty project dir → returns empty slice)
  - [x] `TestIsExecutableJar_WithMainClass` (JAR with `Main-Class` → true)
  - [x] `TestIsExecutableJar_WithoutMainClass` (JAR without `Main-Class` → false)

## 4. Java Process Detection

**Files:** `pkg/installer/otel_runtime_scan.go` (modify), `pkg/installer/otel_runtime_scan_unix.go` (modify), `pkg/installer/otel_runtime_scan_windows.go` (modify), `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

- [x] 4.0 Add `Description string` field to `DetectedProcess` struct in `otel_runtime_scan.go`
- [x] 4.1 Add `enrichProcessesWithJPS(processes []DetectedProcess) []DetectedProcess` — if `jps` is in PATH, run `jps -l`, match output to `ps`-based processes by PID, and populate `DetectedProcess.Description` with the main class or JAR name from `jps`
- [x] 4.2 Add debug logging in `detectJavaProcesses` and `enrichProcessesWithJPS`:
  - After raw scan: `logger.Debug("detected java processes", "count", len(processes))`
  - JPS not found: `logger.Debug("jps not found, skipping enrichment")`
  - Per enriched process: `logger.Debug("jps enrichment", "pid", pid, "description", description)`
  - Per matched process: `logger.Debug("matched process to project", "pid", pid, "project", projectPath)`
  - No matches: `logger.Debug("no running java processes matched to any project")`
- [x] 4.3 Add `detectJavaListeningPort(pid int, projectDir string) string` in `otel_java_process.go` — tries `detectProcessListeningPort(pid)` directly first (handles direct `java -jar` launches); on failure delegates to `javaDescendantPort(pid, projectDir)` for wrapper launchers (mvn, gradle) that fork the app into a separate JVM.
- [x] 4.3a Add `javaDescendantPort(pid int, projectDir string) string` as a platform-specific function in `otel_runtime_scan_unix.go` / `otel_runtime_scan_windows.go`:
  - **Unix**: runs `jps -l`, skips build-tool JVMs via `isBuildToolJVM`, returns the first eligible JVM with a listening port.
  - **Windows**: uses WMI to find a `java.exe` child/descendant of the wrapper PID with a listening port.
- [x] 4.3b Add `isUnderDir(path, dir string) bool` helper in `otel_java_process.go` — returns true when `path` equals `dir` or is directly under it.
- [x] 4.4 Add `isBuildToolJVM(mainClass string) bool` helper to filter Gradle/Maven/jps infrastructure from the `jps -l` fallback.
- [x] 4.5 Add `portDetector func(pid int) string` field to `ManagedProcess` (with `detectPort()` method) so Java launches can inject `detectJavaListeningPort` without touching generic process detection.
- [x] 4.6 Set `proc.portDetector` as a closure `func(pid int) string { return detectJavaListeningPort(pid, path) }` at all three Java `StartManagedProcess` call sites in `otel_java.go` (single-module flow, multi-module `executeMultiModule`, and `InstallOtelJava`).
- [x] 4.7 Tests: `TestIsBuildToolJVM_*` (6 cases), `TestDetectPort_UsesCustomDetector`, `TestDetectPort_FallsBackWithoutDetector`, `TestIsUnderDir` (table-driven, covers equal paths, subdirectories, partial prefix matches, and empty inputs)

## 5. Full InstallOtelJava Automated Flow

**Files:** `pkg/installer/otel_java.go` (modify)

- [x] 5.1 Update `InstallOtelJava` signature to `InstallOtelJava(envURL, token, serviceName string, dryRun bool) error`
- [x] 5.2 Add pre-flight validation call to `validateJavaPrerequisites()` at the top of `InstallOtelJava()`, before any other work
- [x] 5.3 Rewrite the dry-run path to include: API URL, service name, agent JAR download URL, environment variables, and the `-javaagent` JVM flag
- [x] 5.4 Implement the interactive flow:
  1. Detect Java projects via `detectJavaProjects()` and processes via `detectJavaProcesses()`; match processes to projects.
  2. Present project selection menu (with PID annotations where applicable).
  3. Detect entrypoints for the selected project via `detectJavaEntrypoints()`.
  4. If exactly one entrypoint found: auto-select it (no prompt). If multiple entrypoints found: present entrypoint selection menu.
  5. If no entrypoints found: attempt auto-build via `attemptSingleModuleBuild()`; if build fails or no build tool is present, print an error and exit.
  6. Show plan preview (project path, launch command with `-javaagent`, JAR URL, OTEL vars, PIDs to stop).
  7. Confirm with user via `confirmProceed()` — if matched processes exist, prompt text SHALL name them: `Stop PID 1234 (myapp) and proceed with installation?`; otherwise use `Proceed with installation?`
  8. Download the agent JAR.
  9. Stop any running processes matched to the project.
  10. Launch instrumented process via `StartManagedProcess`.
  11. Print process summary via `PrintProcessSummary`.
  12. Call `updateOtelCollectorIfPresent(envURL, token, dryRun)` — probes `<cwd>/opentelemetry/config.yaml`, patches silently with `PatchConfigFile` if found, skips with no output if not found.
  13. Return from `InstallOtelJava()` — `cmd/install.go` calls `WatchIngest` after the installer returns (see task 5.7).
- [x] 5.5 Use `StartManagedProcess` to launch the instrumented process with log file at `<project-path>/<service-name>.log`. Immediately before constructing the `exec.Cmd`, add `logger.Debug("launching instrumented java process", "cmd", launchCmd, "dir", proj.Path)` — this must be the last debug statement before the process starts so the full resolved command is visible when running with `--debug`.
- [x] 5.6 Use `PrintProcessSummary` after the settle period; if no alive processes, output via `display.PrintStatusLine("error", "No services are running — check the logs above for errors.", display.ColorError)`
- [x] 5.7 Post-install ingest watch: handled by `WatchIngest` (from the `ingest_watch` feature), called by `cmd/install.go` after `InstallOtelJava()` returns. The installer does not call it — it only starts the process and returns. Same pattern as every other runtime.
- [x] 5.8 Update `DetectJavaPlan` to build fully executable plans — pass `envURL`, resolved entrypoint command through the `JavaInstrumentationPlan` struct
- [x] 5.9 Update `JavaInstrumentationPlan.Execute()` to use the full automated flow (detect entrypoint → stop → download → launch → update collector)

## 6. Cobra Command Updates

**Files:** `cmd/install.go` (modify), `pkg/installer/otel.go` (modify)

- [x] 6.1 Update `installOtelJavaCmd` RunE in `cmd/install.go` to pass credentials to `installer.InstallOtelJava(envURL, accessTok, otelJavaServiceName, installDryRun)`
- [x] 6.2 Update `createRuntimePlan` for the `"Java"` case to pass `envURL` through to the `JavaInstrumentationPlan`

## 7. Unit Tests for Full Flow

**Files:** `pkg/installer/otel_java_test.go` (modify)

- [x] 7.1 Update `TestDetectJavaPlan_FindsProject` to verify the plan includes the new fields (EnvURL, EntrypointCommand)
- [x] 7.2 Add `TestInstallOtelJava_DryRun` — verify dry-run output includes all expected fields (API URL, service name, agent JAR URL, env vars, `-javaagent` flag)
- [x] 7.3 Add `TestInstallOtelJava_JavaNotFound` — verify error message when Java is not on PATH
- [x] 7.4 Add `TestJavaInstrumentationPlan_PrintPlanSteps_Updated` — verify plan shows launch command with `-javaagent`, JAR download URL, and OTEL vars
- [x] 7.5 Add `TestInstallOtelJava_NoBuildArtifact_NoRunningProcess` — verify that when no JAR exists and no build tool is found, a "no build tool detected" message is printed and no process is started
- [x] 7.6 Add `TestInstallOtelJava_AutoBuildFails` — verify that when a build tool wrapper is present but the build command exits non-zero, `Auto-build failed` is printed and no process is started

## 8. Cross-Platform Wrapper Support

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

**Context:** On Windows, Maven and Gradle wrappers use `.cmd` / `.bat` extensions (`mvnw.cmd`, `gradlew.bat`) and cannot be invoked with a `./` prefix or via `exec.Command` directly — they require `cmd /c`. The current implementation only handles Unix-style wrappers.

- [x] 8.1 Add `"gradlew.bat"` and `"mvnw.cmd"` to `javaProjectMarkers` in `otel_java.go` so Windows-style wrapper presence is recognized as a Java project signal during directory scanning.
- [x] 8.2 Add `findWrapper(projectPath, unixName, windowsName string) string` in `otel_java_process.go` — returns the wrapper filename (not full path) for the current platform: checks `<projectPath>/<windowsName>` on `runtime.GOOS == "windows"`, `<projectPath>/<unixName>` otherwise. Returns `""` if the file does not exist.
- [x] 8.3 Update `detectJavaEntrypoints` to use `findWrapper(projectPath, "mvnw", "mvnw.cmd")` and `findWrapper(projectPath, "gradlew", "gradlew.bat")` instead of hardcoded filenames. Construct platform-correct command strings:
  - Unix: `./mvnw spring-boot:run`, `./gradlew bootRun`, `./gradlew run`
  - Windows: `mvnw.cmd spring-boot:run`, `gradlew.bat bootRun`, `gradlew.bat run`
- [x] 8.4 Update `attemptSingleModuleBuild` to use `findWrapper` for detection and to construct the exec-ready command:
  - Unix: `exec.Command("./mvnw", "clean", "package", "-DskipTests")` / `exec.Command("./gradlew", "build", "-x", "test")` with `cmd.Dir = projectPath`
  - Windows: `exec.Command("cmd", "/c", "mvnw.cmd", "clean", "package", "-DskipTests")` / `exec.Command("cmd", "/c", "gradlew.bat", "build", "-x", "test")` with `cmd.Dir = projectPath`
- [x] 8.5 Update `buildInstrumentedCmd` in `otel_java.go` to handle Windows wrapper commands: when `fields[0]` ends in `.cmd` or `.bat`, wrap the execution as `exec.Command("cmd", append([]string{"/c", fields[0]}, fields[1:]...)...)` so the wrapper executes correctly via Windows command processor.
- [x] 8.6 Tests in `otel_java_process_test.go`:
  - [x] `TestFindWrapper_FoundOnCurrentPlatform` — temp dir with both `mvnw` and `mvnw.cmd` present; verify the correct one is returned for the current `runtime.GOOS`
  - [x] `TestFindWrapper_Missing_ReturnsEmpty` — temp dir with no wrapper → returns `""`
  - [x] `TestDetectJavaEntrypoints_WindowsWrapperSpringBootMaven` — temp dir with `mvnw.cmd` + Spring Boot `pom.xml`, no JAR → returns candidate whose `Command` starts with `mvnw.cmd` (skip on non-Windows with `t.Skip`)
  - [x] `TestDetectJavaEntrypoints_WindowsWrapperSpringBootGradle` — temp dir with `gradlew.bat` + Spring Boot `build.gradle`, no JAR → returns candidate whose `Command` starts with `gradlew.bat` (skip on non-Windows with `t.Skip`)

## 9. Verification

### Automated (run on both platforms)

- [x] 9.1 Run `make test` on Unix — all existing tests must pass
- [x] 9.1a Run `go test ./...` on Windows — all existing tests must pass (Windows-skipped tests are acceptable; no panics or build failures)
- [x] 9.2 Run `make lint` — no new lint issues

### Manual — Unix (macOS / Linux)

- [x] 9.3 `dtwiz install otel-java --dry-run` shows preview with JAR URL, env vars, and `-javaagent` flag
- [x] 9.4 `dtwiz install otel-java` with a Java project that has a built fat JAR — JAR is detected as entrypoint, app is launched with instrumentation (no prior running process needed)
- [x] 9.5 `dtwiz install otel-java` with no built artifact — installer attempts auto-build via `./mvnw` or `./gradlew`; if build succeeds the app is launched; if build fails a clear error is printed with instructions to fix and re-run
- [x] 9.6 Generate some traffic to the instrumented app and verify traces/logs appear in Dynatrace
- [x] 9.7 `dtwiz install otel` shows Java projects in the selection menu (requires `DTWIZ_ALL_RUNTIMES=true` until task 14 is complete)
- [x] 9.8 After `dtwiz install otel-java` completes, `WatchIngest` starts automatically and shows ingested data (services, logs, spans) for the instrumented Java app; press Enter to exit the watch
- [x] 9.9 OTel Collector config is updated after Java instrumentation when a collector config exists on the machine
- [x] 9.9a `dtwiz install otel-java` inside `terra-sample-apps/java-travel-agency` — all 5 services (`s-frontend`, `s-frontend-2`, `s-booking`, `s-transport`, `s-load-balancer`) start, each with a distinct `OTEL_SERVICE_NAME` matching its directory name; zero services start via the root `./mvnw spring-boot:run` command

### Manual — Windows

- [x] 9.10 `dtwiz install otel-java --dry-run` shows preview with correct Windows paths (backslash separators in agent JAR path; home resolves under `%USERPROFILE%`)
- [x] 9.11 `dtwiz install otel-java` with a fat JAR project — JAR detected, instrumented process launched
- [x] 9.12 `dtwiz install otel-java` with a Spring Boot Maven project using `mvnw.cmd` — `mvnw.cmd spring-boot:run` offered as entrypoint and executes correctly with `JAVA_TOOL_OPTIONS` set
- [x] 9.13 `dtwiz install otel-java` with a Spring Boot Gradle project using `gradlew.bat` — `gradlew.bat bootRun` offered and executes correctly
- [x] 9.14 `dtwiz install otel-java` with no built artifact — auto-build via `mvnw.cmd` or `gradlew.bat` is attempted; success launches; failure prints clear error
- [x] 9.15 Running Java processes detected and shown in project selection menu with PID annotations
- [x] 9.16 `dtwiz uninstall otel` stops the dtwiz-instrumented Java process and removes `%USERPROFILE%\.opentelemetry\java\`

## 10. Multi-Module Project Detection and Instrumentation

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java.go` (modify), `pkg/installer/otel.go` (modify)

- [x] 10.1 Add `isMavenMultiModule(projectPath string) bool` — parse root `pom.xml` via `encoding/xml` and return true when `<modules>` is non-empty
- [x] 10.2 Add `parseMavenModules(projectPath string) ([]string, error)` — extract `<module>` entries from root `pom.xml`; return nil/empty for non-multi-module projects
- [x] 10.3 Add `isGradleMultiProject(projectPath string) bool` and `parseGradleSubprojects(projectPath string) ([]string, error)` — regex scan `settings.gradle` / `settings.gradle.kts` for `include` directives; convert colon notation to path separators
- [x] 10.4 Add `mavenBuildCommand(projectPath string) string` and `gradleBuildCommand(projectPath string) string` — return the build command based on which wrapper is present, or `""` if none
- [x] 10.5 Add `needsBuild(subs []SubModule) bool` — return true when any sub-module is missing a fat JAR in `target/` or `build/libs/`
- [x] 10.6 Add `detectMultiModule(projectPath string) *MultiModuleProject` — checks Maven first, then Gradle; returns `nil` for single-module projects
- [x] 10.7 Add `SubModulePlan` struct with `Name`, `Path`, `LaunchCommand`, `EnvVars` fields
- [x] 10.8 Add `BuildCommand string` and `SubModules []SubModulePlan` fields to `JavaInstrumentationPlan`
- [x] 10.9 Add `buildMultiModulePlan(mm *MultiModuleProject, proj ScannedProject, ...) *JavaInstrumentationPlan` — constructs a full plan with per-module env vars and (pre-build) launch commands
- [x] 10.10 Add `executeMultiModule()` method — runs build (if `BuildCommand` is set), refreshes launch commands from newly-built JARs, launches each module as a separate `ManagedProcess`, calls `PrintProcessSummary` with all alive services. Ingest watch is handled by the CLI layer after the installer returns (see task 5.7).
- [x] 10.11 Update `DetectJavaPlan()` to call `detectMultiModule()` before single-module entrypoint detection
- [x] 10.12 Update `InstallOtelJava()` to call `detectMultiModule()` after project selection; show multi-module plan preview with build command and per-module launch commands
- [x] 10.13 Update `createRuntimePlan()` in `otel.go` for the Java case to call `detectMultiModule()` and resolve single-module entrypoints at plan time (not deferred to `Execute()`)
- [x] 10.14 Ensure `Execute()` dispatches to `executeMultiModule()` when `SubModules` is non-empty — the multi-runtime flow calls `Execute()`, not `executeMultiModule()` directly; without this guard, multi-module plans silently fell through to the single-module path and started only one process

## 11. Entrypoint Resolution Before Preview

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_java.go` (modify)

- [x] 11.1 In `createRuntimePlan()` Java case: call `detectJavaEntrypoints()` + `promptEntrypointSelection()` at plan time and store result in `EntrypointCommand` — the preview SHALL always show the resolved command
- [x] 11.2 Update `PrintPlanSteps()` to show `(entrypoint will be detected at execution time)` only as a last-resort fallback, not as the default for unresolved entrypoints
- [x] 11.3 Remove all uses of `java -javaagent:... -jar your_app.jar` placeholder text from non-instruction contexts

## 12. Unit Tests for Multi-Module Detection

**Files:** `pkg/installer/otel_java_process_test.go` (modify)

- [x] 12.1 `TestIsMavenMultiModule_MultiModule` — temp dir with multi-module `pom.xml` → true
- [x] 12.2 `TestIsMavenMultiModule_SingleModule` — temp dir with single-module `pom.xml` → false
- [x] 12.3 `TestParseMavenModules_ReturnsModuleNames` — verify all `<module>` entries are extracted
- [x] 12.4 `TestParseGradleSubprojects_ColonNotation` — `include ':api'` → `["api"]`
- [x] 12.5 `TestParseGradleSubprojects_NestedPath` — `include ':ui:web'` → `["ui/web"]`
- [x] 12.6 `TestDetectMultiModule_Maven` — returns correct `MultiModuleProject` for Maven multi-module project
- [x] 12.7 `TestDetectMultiModule_NilForSingleModule` — returns nil for single-module project
- [x] 12.8 `TestNeedsBuild_TrueWhenJarsMissing` — returns true when sub-module has no JAR
- [x] 12.9 `TestNeedsBuild_FalseWhenJarsPresent` — returns false when all sub-modules have JARs
- [x] 12.10 `TestJavaInstrumentationPlan_Execute_MultiModuleDispatch` — plan with `SubModules` set but no JARs in sub-dirs; verifies agent download is attempted (multi-module path taken) and output contains both sub-module names; verifies single-module error message ("no runnable entrypoint detected") is absent
- [x] 12.11 `TestDetectJavaPlan_MultiModule_HasSubModules` — temp Maven multi-module root (`pom.xml` with two `<module>` entries); verifies `DetectJavaPlan` returns a plan with 2 `SubModules` and no `EntrypointCommand`

## 13. Extend `uninstall otel` with Java Cleanup

**Files:** `pkg/installer/otel_uninstall.go` (modify), `pkg/installer/otel_uninstall_test.go` (modify or create)

- [x] 13.1 Add `findInstrumentedJavaProcesses() []DetectedProcess` in `otel_uninstall.go` — calls `detectJavaProcesses()` + `enrichProcessesWithJPS()`, filters to processes whose `Command` contains the dtwiz agent path OR whose open file descriptors include the agent JAR (`jvmHasAgentLoaded`). The second check catches Gradle `bootRun` where the agent is injected via `JAVA_TOOL_OPTIONS` and does not appear in the command line.
- [x] 13.2 Add `javaAgentDir() string` helper — returns `filepath.Dir(javaAgentPath())`
- [x] 13.3 Extend `UninstallOtelCollector(dryRun bool) error` to include a Java cleanup section: discover instrumented Java processes and the agent dir, include them in the combined preview alongside existing collector artifacts, and on confirmation stop matched processes then remove `~/.opentelemetry/java/` if it exists
- [x] 13.4 Tests:
  - `TestFindInstrumentedJavaProcesses_FiltersByAgentFlag` — verify only processes with `opentelemetry-javaagent.jar` in command are returned
  - `TestFindInstrumentedJavaProcesses_NoneMatching` — verify empty result when no processes have the agent flag
  - `TestJavaAgentDir_ReturnsParentOfJar` — verify the directory name ends in `java`
  - `TestUninstallOtelCollector_JavaDryRun_NothingPresent` — no Java processes, no agent dir → Java section absent from preview
  - `TestUninstallOtelCollector_JavaDryRun_AgentDirExists` — agent dir present, dry-run → dir not removed
- [x] 13.5 Manual verification: `dtwiz uninstall otel --dry-run` with a running instrumented Java process — shows Java PID and agent dir in preview, makes no changes
- [x] 13.6 Manual verification: `dtwiz uninstall otel` stops only the dtwiz-instrumented Java process, not other Java processes
- [x] 13.7 Manual verification: `dtwiz uninstall otel` with no running Java processes but agent JAR present — removes `~/.opentelemetry/java/` only
- [x] 13.8 Manual verification: `dtwiz uninstall otel` with nothing Java-related to remove — Java section is absent from output; existing collector behavior unchanged

## 14. Remove DTWIZ_ALL_RUNTIMES Gate

**Do this only after all other tasks are complete and verified.**

**Files:** `pkg/installer/otel.go` (modify)

- [x] 14.1 In `detectAvailableRuntimes()`, set `enabled: true` for Java unconditionally (remove the `allRuntimesEnabled()` gate)
- [x] 14.2 Remove the "Coming soon" label from the Java entry in the runtime list (if present in the display output)
