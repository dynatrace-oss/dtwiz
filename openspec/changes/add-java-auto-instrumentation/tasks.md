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

## 3. Java Process Detection and Command Reconstruction

**Files:** `pkg/installer/otel_java_process.go` (modify), `pkg/installer/otel_java_process_test.go` (modify)

- [ ] 3.1 Add `detectJavaProcessesWithJPS() []DetectedProcess` — if `jps` is in PATH, run `jps -l` and parse output to get PID → main class mapping. Return enriched `DetectedProcess` entries.
- [ ] 3.2 Add `enrichProcessesWithJPS(processes []DetectedProcess) []DetectedProcess` — match `jps` output to `ps`-based processes by PID, add main class name to the process description for the selection menu.
- [ ] 3.3 Add `reconstructJavaCommand(command string, agentJarPath string) (string, error)` — parse the command string and insert `-javaagent:<path>` flag. Handle patterns: `-jar`, `-cp`/`-classpath`, `-m`/`--module`, direct class name. Replace existing `-javaagent:…opentelemetry-javaagent.jar` if present. Return error for unrecognized patterns.
- [ ] 3.4 Add `parseJavaCommandParts(command string) (javaBin string, jvmFlags []string, appArgs []string, launchType string)` — split a `ps`-output Java command into its components for reconstruction.
- [ ] 3.5 Tests in `pkg/installer/otel_java_process_test.go`:
  - `TestReconstructJavaCommand_JarPattern` (input: `java -Xmx512m -jar app.jar --port 8080` → inserts `-javaagent` before `-jar`)
  - `TestReconstructJavaCommand_ClasspathPattern` (input: `java -cp lib/*:. com.example.Main` → inserts before `-cp`)
  - `TestReconstructJavaCommand_ModulePattern` (input: `java -m com.example/Main` → inserts before `-m`)
  - `TestReconstructJavaCommand_DirectClass` (input: `java com.example.Main` → inserts `-javaagent` after `java`)
  - `TestReconstructJavaCommand_AlreadyInstrumented` (replaces existing `-javaagent`)
  - `TestReconstructJavaCommand_UnrecognizedPattern` (input: `/opt/tomcat/bin/catalina.sh run` → error)
  - `TestReconstructJavaCommand_PreservesExistingJVMFlags` (input: `java -Xmx1g -XX:+UseG1GC -jar app.jar` → preserves all flags)
  - `TestParseJavaCommandParts_VariousPatterns` (table-driven test for different command structures)

## 4. Full InstallOtelJava Automated Flow

**Files:** `pkg/installer/otel_java.go` (modify)

- [ ] 4.1 Update `InstallOtelJava` signature to `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error`
- [ ] 4.2 Add pre-flight validation call to `validateJavaPrerequisites()` at the top of `InstallOtelJava()`, before any other work
- [ ] 4.3 Rewrite the dry-run path to include: API URL, service name, agent JAR download URL, environment variables (with `OTEL_EXPORTER_OTLP_PROTOCOL` and `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE`), and the `-javaagent` JVM flag
- [ ] 4.4 Implement the interactive flow: detect projects → detect processes → select process → reconstruct command → show plan preview → confirm → download JAR → stop process → launch instrumented process → print summary → `waitForServices()`
- [ ] 4.5 Use `StartManagedProcess` to launch the instrumented process with log file at `<project-path>/<service-name>.log`
- [ ] 4.6 Use `PrintProcessSummary` after the settle period; if no alive processes, print "No services are running — check the logs above for errors." and skip `waitForServices`
- [ ] 4.7 Call `waitForServices(envURL, platformToken, aliveServiceNames)` when at least one process is alive
- [ ] 4.8 Update `DetectJavaPlan` to build fully executable plans (not manual-instruction plans) — pass `envURL`, `platformToken` through the `JavaInstrumentationPlan` struct
- [ ] 4.9 Update `JavaInstrumentationPlan.Execute()` to use the full automated flow (stop → download → launch)

## 5. Cobra Command and Runtime Gate Updates

**Files:** `cmd/install.go` (modify), `pkg/installer/otel.go` (modify)

- [ ] 5.1 Update `installOtelJavaCmd` RunE in `cmd/install.go` to pass `platformTok` to `installer.InstallOtelJava(envURL, accessTok, platformTok, otelJavaServiceName, installDryRun)`
- [ ] 5.2 In `pkg/installer/otel.go`, change `detectAvailableRuntimes()` to set `enabled: true` for Java (remove the `allEnabled` gate)
- [ ] 5.3 Update `createRuntimePlan` for the `"Java"` case to pass `envURL` and `platformToken` through to the `JavaInstrumentationPlan`

## 6. Unit Tests for Full Flow

**Files:** `pkg/installer/otel_java_test.go` (modify)

- [ ] 6.1 Update `TestDetectJavaPlan_FindsProject` to verify the plan includes the new fields (EnvURL, PlatformToken)
- [ ] 6.2 Add `TestInstallOtelJava_DryRun` — verify dry-run output includes all expected fields (API URL, service name, agent JAR URL, env vars, `-javaagent` flag)
- [ ] 6.3 Add `TestInstallOtelJava_JavaNotFound` — verify error message when Java is not on PATH
- [ ] 6.4 Add `TestJavaInstrumentationPlan_PrintPlanSteps_Updated` — verify plan shows process PID, JAR download, and `-javaagent` instruction

## 7. Integration Testing and Verification

- [ ] 7.1 Run `make test` — all existing tests must pass
- [ ] 7.2 Run `make lint` — no new lint issues
- [ ] 7.3 Manual verification: `dtwiz install otel-java --dry-run` shows preview with JAR URL, env vars, and `-javaagent` flag
- [ ] 7.4 Manual verification: `dtwiz install otel-java` with a running Java app (e.g., java-travel-agency) — JAR is downloaded, process is detected, user selects process, process restarts with instrumentation
- [ ] 7.5 Manual verification: generate some traffic to the instrumented app and verify traces/logs appear in Dynatrace
- [ ] 7.6 Manual verification: `dtwiz install otel` shows Java projects in the selection menu without `DTWIZ_ALL_RUNTIMES`
- [ ] 7.7 Manual verification: "Waiting for traffic" terminates when service appears in Dynatrace (not just on timeout)
- [ ] 7.8 Manual verification: process crash during startup shows `[crashed: ...]` summary and skips the traffic-waiting prompt
