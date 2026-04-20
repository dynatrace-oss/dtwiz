# Tasks

## 1. Java Version Validation

**Files:** `pkg/installer/otel_java_process.go` (create), `pkg/installer/otel_java_process_test.go` (create)

- [ ] 1.1 Create `pkg/installer/otel_java_process.go` with `parseJavaVersion(output string) (int, error)` — extract the quoted version string from `java -version` stderr; handle legacy (`1.8.0_382` → 8) and modern (`17.0.1` → 17, `21` → 21) formats
- [ ] 1.2 Add `validateJavaPrerequisites() (string, error)` — check `java` in PATH via `exec.LookPath`, run `java -version`, parse the output, return error if version < 8. Return the java binary path on success.
- [ ] 1.3 Tests in `pkg/installer/otel_java_process_test.go`:
  - `TestParseJavaVersion_Legacy_1_8` (input: `openjdk version "1.8.0_382"` → 8)
  - `TestParseJavaVersion_Modern_17` (input: `java version "17.0.1" 2021-10-19` → 17)
  - `TestParseJavaVersion_Short_21` (input: `openjdk version "21" 2023-09-19` → 21)
  - `TestParseJavaVersion_OpenJDK_11` (input: `openjdk version "11.0.20" 2023-07-18` → 11)
  - `TestParseJavaVersion_Unrecognized` (input: `not a valid version` → error)
  - `TestParseJavaVersion_Java7_TooOld` (input: `java version "1.7.0_80"` → 7, then validate rejects it)

## 2. Agent JAR Download

**Files:** `pkg/installer/otel_java.go` (modify)

- [ ] 2.1 Implement `downloadJavaAgent() (string, error)` — download the JAR from `otelJavaAgentURL` to `~/opentelemetry/java/opentelemetry-javaagent.jar`. Create the directory if it does not exist. Use `net/http.Get` + `os.Create` + `io.Copy`. Return the absolute path to the JAR.
- [ ] 2.2 Handle download errors: non-200 HTTP status → return error with URL and status code; network errors → return error with URL and error message.
- [ ] 2.3 Print download progress: `Downloading OpenTelemetry Java agent... done.`
- [ ] 2.4 Tests in `pkg/installer/otel_java_test.go`:
  - `TestDownloadJavaAgent_CreatesDirectory` (verify directory creation with temp dir)
  - `TestDownloadJavaAgent_ErrorOnNon200` (mock HTTP response)

## 3. Java Entrypoint Detection

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

- [ ] 3.1 Add `detectJavaEntrypoints(projectPath string) []JavaEntrypoint` — scan for runnable artifacts in the project directory. A `JavaEntrypoint` has `Command string` (the full launch command) and `Description string` (shown in the selection menu).
  - Scan `target/*.jar` and `build/libs/*.jar` for JARs with a `Main-Class` in `MANIFEST.MF` (use `archive/zip` to read the JAR).
  - If no fat JAR is found, check for build-tool wrappers with the following rules:
    - **Maven:** only offer a wrapper if `pom.xml` references `spring-boot` → `./mvnw spring-boot:run`. `exec:java` is never offered (requires `mainClass` POM config absent in most projects).
    - **Gradle Spring Boot:** if `build.gradle`/`build.gradle.kts` references `springframework.boot` or `spring-boot` → `./gradlew bootRun`.
    - **Gradle generic:** `./gradlew run` for non-Spring Boot Gradle projects.
    - **Maven non-Spring Boot with no JAR:** no wrapper offered — fall through to build instructions.
  - Add `isSpringBootMaven(projectPath string) bool` — reads `pom.xml` and checks for `spring-boot` substring.
  - Add `isSpringBootGradle(projectPath string) bool` — reads `build.gradle`/`build.gradle.kts` and checks for `spring-boot` or `springframework.boot` substrings.
- [ ] 3.2 Add `isExecutableJar(jarPath string) bool` — open the JAR as a ZIP, read `META-INF/MANIFEST.MF`, return true if `Main-Class:` is present.
- [ ] 3.3 Add `promptEntrypointSelection(entrypoints []JavaEntrypoint) *JavaEntrypoint` — when exactly one entrypoint is found, auto-select it (print the selection, no prompt); when multiple are found, present a numbered menu; return nil if user skips.
- [ ] 3.4 Tests in `pkg/installer/otel_java_process_test.go`:
  - `TestDetectJavaEntrypoints_MavenFatJar` (temp dir with `target/app.jar` containing `Main-Class` → returns jar candidate)
  - `TestDetectJavaEntrypoints_GradleFatJar` (temp dir with `build/libs/app-all.jar` → returns jar candidate)
  - `TestDetectJavaEntrypoints_MavenWrapperSpringBoot` (temp dir with `mvnw` + Spring Boot `pom.xml` → returns `spring-boot:run` candidate)
  - `TestDetectJavaEntrypoints_MavenWrapperNonSpringBoot` (temp dir with `mvnw` + plain `pom.xml` → returns no entrypoint)
  - `TestDetectJavaEntrypoints_GradleWrapperSpringBoot` (temp dir with `gradlew` + Spring Boot `build.gradle` → returns `bootRun` candidate)
  - `TestDetectJavaEntrypoints_GradleWrapperNoJar` (temp dir with `gradlew`, no Spring Boot → returns `run` candidate)
  - `TestDetectJavaEntrypoints_NoEntrypoint` (empty project dir → returns empty slice)
  - `TestIsExecutableJar_WithMainClass` (JAR with `Main-Class` → true)
  - `TestIsExecutableJar_WithoutMainClass` (JAR without `Main-Class` → false)

## 4. Java Process Detection

**Files:** `pkg/installer/otel_runtime_scan.go` (modify), `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

- [ ] 4.0 Add `Description string` field to `DetectedProcess` struct in `otel_runtime_scan.go`
- [ ] 4.1 Add `enrichProcessesWithJPS(processes []DetectedProcess) []DetectedProcess` — if `jps` is in PATH, run `jps -l`, match output to `ps`-based processes by PID, and populate `DetectedProcess.Description` with the main class or JAR name from `jps`

## 5. Full InstallOtelJava Automated Flow

**Files:** `pkg/installer/otel_java.go` (modify)

- [ ] 5.1 Update `InstallOtelJava` signature to `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error`
- [ ] 5.2 Add pre-flight validation call to `validateJavaPrerequisites()` at the top of `InstallOtelJava()`, before any other work
- [ ] 5.3 Rewrite the dry-run path to include: API URL, service name, agent JAR download URL, environment variables, and the `-javaagent` JVM flag
- [ ] 5.4 Implement the interactive flow:
  1. Detect Java projects via `detectJavaProjects()` and processes via `detectJavaProcesses()`; match processes to projects.
  2. Present project selection menu (with PID annotations where applicable).
  3. Detect entrypoints for the selected project via `detectJavaEntrypoints()`.
  4. If exactly one entrypoint found: auto-select it (no prompt). If multiple entrypoints found: present entrypoint selection menu.
  5. If no entrypoints found: print build instructions + manual `-javaagent` steps and exit.
  6. Show plan preview (project path, launch command with `-javaagent`, JAR URL, OTEL vars, PIDs to stop).
  7. Confirm with user via `confirmProceed()` — if matched processes exist, prompt text SHALL name them: `Stop PID 1234 (myapp) and proceed with installation?`; otherwise use `Proceed with installation?`
  8. Download the agent JAR.
  9. Stop any running processes matched to the project.
  10. Launch instrumented process via `StartManagedProcess`.
  11. Print process summary via `PrintProcessSummary`.
  12. Call `updateOtelCollectorIfPresent(envURL, token, dryRun)` — probes `<cwd>/opentelemetry/config.yaml`, patches silently with `PatchConfigFile` if found, skips with no output if not found.
  13. Call `waitForServices()` if at least one process is alive.
- [ ] 5.5 Use `StartManagedProcess` to launch the instrumented process with log file at `<project-path>/<service-name>.log`. Before building the `exec.Cmd`, add `logger.Debug("launching instrumented java process", "cmd", launchCmd, "dir", proj.Path)` so the full command is visible in debug output.
- [ ] 5.6 Use `PrintProcessSummary` after the settle period; if no alive processes, print "No services are running — check the logs above for errors." and skip `waitForServices`
- [ ] 5.7 Call `waitForServices(envURL, platformToken, aliveServiceNames)` when at least one process is alive
- [ ] 5.8 Update `DetectJavaPlan` to build fully executable plans — pass `envURL`, `platformToken`, resolved entrypoint command through the `JavaInstrumentationPlan` struct
- [ ] 5.9 Update `JavaInstrumentationPlan.Execute()` to use the full automated flow (detect entrypoint → stop → download → launch → update collector)

## 6. Cobra Command Updates

**Files:** `cmd/install.go` (modify), `pkg/installer/otel.go` (modify)

- [ ] 6.1 Update `installOtelJavaCmd` RunE in `cmd/install.go` to pass `platformTok` to `installer.InstallOtelJava(envURL, accessTok, platformTok, otelJavaServiceName, installDryRun)`
- [ ] 6.2 Update `createRuntimePlan` for the `"Java"` case to pass `envURL` and `platformToken` through to the `JavaInstrumentationPlan`

## 7. Unit Tests for Full Flow

**Files:** `pkg/installer/otel_java_test.go` (modify)

- [ ] 7.1 Update `TestDetectJavaPlan_FindsProject` to verify the plan includes the new fields (EnvURL, PlatformToken, EntrypointCommand)
- [ ] 7.2 Add `TestInstallOtelJava_DryRun` — verify dry-run output includes all expected fields (API URL, service name, agent JAR URL, env vars, `-javaagent` flag)
- [ ] 7.3 Add `TestInstallOtelJava_JavaNotFound` — verify error message when Java is not on PATH
- [ ] 7.4 Add `TestJavaInstrumentationPlan_PrintPlanSteps_Updated` — verify plan shows launch command with `-javaagent`, JAR download URL, and OTEL vars
- [ ] 7.5 Add `TestInstallOtelJava_NoBuildArtifact_NoRunningProcess` — verify fallback message with build instructions is printed and no process is started

## 8. Remove DTWIZ_ALL_RUNTIMES Gate

**Do this only after all tasks in sections 1–7 are complete and verified.**

**Files:** `pkg/installer/otel.go` (modify)

- [ ] 8.1 In `detectAvailableRuntimes()`, set `enabled: true` for Java unconditionally (remove the `allRuntimesEnabled()` gate)
- [ ] 8.2 Remove the "Coming soon" label from the Java entry in the runtime list (if present in the display output)

## 10. Multi-Module Project Detection and Instrumentation

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java.go` (modify), `pkg/installer/otel.go` (modify)

- [ ] 10.1 Add `isMavenMultiModule(projectPath string) bool` — parse root `pom.xml` via `encoding/xml` and return true when `<modules>` is non-empty
- [ ] 10.2 Add `parseMavenModules(projectPath string) ([]string, error)` — extract `<module>` entries from root `pom.xml`; return nil/empty for non-multi-module projects
- [ ] 10.3 Add `isGradleMultiProject(projectPath string) bool` and `parseGradleSubprojects(projectPath string) ([]string, error)` — regex scan `settings.gradle` / `settings.gradle.kts` for `include` directives; convert colon notation to path separators
- [ ] 10.4 Add `mavenBuildCommand(projectPath string) string` and `gradleBuildCommand(projectPath string) string` — return the build command based on which wrapper is present, or `""` if none
- [ ] 10.5 Add `needsBuild(subs []SubModule) bool` — return true when any sub-module is missing a fat JAR in `target/` or `build/libs/`
- [ ] 10.6 Add `detectMultiModule(projectPath string) *MultiModuleProject` — checks Maven first, then Gradle; returns `nil` for single-module projects
- [ ] 10.7 Add `SubModulePlan` struct with `Name`, `Path`, `LaunchCommand`, `EnvVars` fields
- [ ] 10.8 Add `BuildCommand string` and `SubModules []SubModulePlan` fields to `JavaInstrumentationPlan`
- [ ] 10.9 Add `buildMultiModulePlan(mm *MultiModuleProject, proj ScannedProject, ...) *JavaInstrumentationPlan` — constructs a full plan with per-module env vars and (pre-build) launch commands
- [ ] 10.10 Add `executeMultiModule()` method — runs build (if `BuildCommand` is set), refreshes launch commands from newly-built JARs, launches each module as a separate `ManagedProcess`, calls `PrintProcessSummary` and `waitForServices` with all alive services
- [ ] 10.11 Update `DetectJavaPlan()` to call `detectMultiModule()` before single-module entrypoint detection
- [ ] 10.12 Update `InstallOtelJava()` to call `detectMultiModule()` after project selection; show multi-module plan preview with build command and per-module launch commands
- [ ] 10.13 Update `createRuntimePlan()` in `otel.go` for the Java case to call `detectMultiModule()` and resolve single-module entrypoints at plan time (not deferred to `Execute()`)

## 11. Entrypoint Resolution Before Preview

**Files:** `pkg/installer/otel.go` (modify), `pkg/installer/otel_java.go` (modify)

- [ ] 11.1 In `createRuntimePlan()` Java case: call `detectJavaEntrypoints()` + `promptEntrypointSelection()` at plan time and store result in `EntrypointCommand` — the preview SHALL always show the resolved command
- [ ] 11.2 Update `PrintPlanSteps()` to show `(entrypoint will be detected at execution time)` only as a last-resort fallback, not as the default for unresolved entrypoints
- [ ] 11.3 Remove all uses of `java -javaagent:... -jar your_app.jar` placeholder text from non-instruction contexts

## 12. Unit Tests for Multi-Module Detection

**Files:** `pkg/installer/otel_java_process_test.go` (modify)

- [ ] 12.1 `TestIsMavenMultiModule_MultiModule` — temp dir with multi-module `pom.xml` → true
- [ ] 12.2 `TestIsMavenMultiModule_SingleModule` — temp dir with single-module `pom.xml` → false
- [ ] 12.3 `TestParseMavenModules_ReturnsModuleNames` — verify all `<module>` entries are extracted
- [ ] 12.4 `TestParseGradleSubprojects_ColonNotation` — `include ':api'` → `["api"]`
- [ ] 12.5 `TestParseGradleSubprojects_NestedPath` — `include ':ui:web'` → `["ui/web"]`
- [ ] 12.6 `TestDetectMultiModule_Maven` — returns correct `MultiModuleProject` for Maven multi-module project
- [ ] 12.7 `TestDetectMultiModule_NilForSingleModule` — returns nil for single-module project
- [ ] 12.8 `TestNeedsBuild_TrueWhenJarsMissing` — returns true when sub-module has no JAR
- [ ] 12.9 `TestNeedsBuild_FalseWhenJarsPresent` — returns false when all sub-modules have JARs

- [ ] 9.1 Run `make test` — all existing tests must pass
- [ ] 9.2 Run `make lint` — no new lint issues
- [ ] 9.3 Manual verification: `dtwiz install otel-java --dry-run` shows preview with JAR URL, env vars, and `-javaagent` flag
- [ ] 9.4 Manual verification: `dtwiz install otel-java` with a Java project that has a built fat JAR — JAR is detected as entrypoint, app is launched with instrumentation (no prior running process needed)
- [ ] 9.5 Manual verification: `dtwiz install otel-java` with no built artifact and no running process — prints build instructions and manual `-javaagent` steps, exits cleanly
- [ ] 9.6 Manual verification: generate some traffic to the instrumented app and verify traces/logs appear in Dynatrace
- [ ] 9.7 Manual verification: `dtwiz install otel` shows Java projects in the selection menu without `DTWIZ_ALL_RUNTIMES`
- [ ] 9.8 Manual verification: "Waiting for traffic" terminates when service appears in Dynatrace (not just on timeout)
- [ ] 9.9 Manual verification: OTel Collector config is updated after Java instrumentation when a collector config exists on the machine
