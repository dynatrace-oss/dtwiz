<!-- Parallelism: Groups 1, 2, 3 are independent — can be developed in parallel. Group 4 depends on 1+2+3. Group 5 is independent of 4. -->

## 1. Function Signature & Pre-flight Validation

- [ ] 1.1 Update `InstallOtelJava` signature to `InstallOtelJava(envURL, token, platformToken, serviceName string, dryRun bool) error` — add `platformToken` parameter needed for DQL verification
- [ ] 1.2 Update `cmd/install.go` to pass `platformTok` to `InstallOtelJava()` (currently only passes `envURL`, `accessTok`, `serviceName`, `installDryRun`)
- [ ] 1.3 Extend `generateOtelJavaEnvVars()` to include `OTEL_EXPORTER_OTLP_PROTOCOL: "http/protobuf"` and `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE: "delta"` — match the Python env var set
- [ ] 1.4 Extend `detectJava()` in `otel_java.go` to parse version from `java -version` output and return `fmt.Errorf` if < Java 8
- [ ] 1.5 Call validation at the start of `InstallOtelJava()` — return error if checks fail (errors bubble to cmd layer, no `os.Exit`)
- [ ] 1.6 Add unit tests for version parsing (test against `openjdk version "1.8.0_..."`, `java version "17.0.1"`, etc.)

## 2. Agent JAR Download

- [ ] 2.1 Implement `downloadJavaAgent(destDir string) (string, error)`: follow redirect from `https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest` (same pattern as `otelLatestReleaseVersion` in `otel_collector.go` — use `http.Client{CheckRedirect: ...}` to avoid API rate limits), download JAR to `destDir/opentelemetry-javaagent.jar`
- [ ] 2.2 Handle existing JAR: check if file exists at target path, prompt user via `confirmProceed()` to re-download or use existing
- [ ] 2.3 Handle download failure: return `fmt.Errorf` with the manual download URL (`otelJavaAgentURL` constant already exists)
- [ ] 2.4 On macOS: call `macOSPrepBinary()` or equivalent to remove quarantine attributes from the JAR
- [ ] 2.5 Add unit tests for release URL resolution logic

## 3. Java Process Detection

- [ ] 3.1 Implement `detectJavaProcesses() []JavaProcess` type and function: use `exec.Command("ps", "ax", "-o", "pid=,command=")` (same pattern as `detectPythonProcesses()`), filter for lines containing `java`, parse PID and full command, resolve CWD via `lsof -a -d cwd -p {pid} -Fn`
- [ ] 3.2 If `exec.LookPath("jps")` succeeds, use `jps -l` as supplementary source to enrich main class names
- [ ] 3.3 Present detected processes as a numbered selection menu using `bufio.Scanner` for input (same pattern as Python project selection)
- [ ] 3.4 Handle no-processes case: print env vars and JVM flag instructions (fall back to current manual-instructions behavior using `GenerateEnvExportScript()`)
- [ ] 3.5 Add unit tests for `ps` output parsing logic

## 4. Instrumented Launch (depends on 1, 2, 3)

- [ ] 4.1 Implement `instrumentJavaProcess()`: parse the original command from the detected process, insert `-javaagent:{jarPath}` after `java` and before the main class/JAR arg, set OTEL_* env vars via `cmd.Env = append(os.Environ(), ...)`
- [ ] 4.2 Show preview: print the full instrumented command and env vars using the same separator/color pattern as `collectorPlan.printConfigPreview()`, prompt via `confirmProceed("Apply?")`
- [ ] 4.3 On confirm: stop the original process via `stopProcesses()` (reuse from `otel_python.go` — sends SIGINT), launch the instrumented command via `exec.Command().Start()` with `cmd.Process.Release()` for detachment (same pattern as `startOtelCollector()`)
- [ ] 4.4 After launch: call `waitForServices(envURL, platformToken, []string{serviceName})` to verify service appears in Dynatrace (reuse from `otel_python.go`)
- [ ] 4.5 In dry-run mode: print the instrumented command, env vars, and JVM flags without executing — return early after printing

## 5. Uninstall Command (independent of group 4)

- [ ] 5.1 Add `otel-java` subcommand to `uninstallCmd` in `cmd/uninstall.go` — follow existing pattern: `Args: cobra.NoArgs`, `RunE` calls `installer.UninstallOtelJava(uninstallDryRun)`
- [ ] 5.2 Create `pkg/installer/otel_java_uninstall.go` with `UninstallOtelJava(dryRun bool) error`
- [ ] 5.3 Implement: find processes with `-javaagent:` + `opentelemetry-javaagent.jar` in their command via `ps ax`, list them + `~/opentelemetry/java/` directory, show preview using `confirmProceed()`, stop processes via `stopProcesses()`, remove directory via `os.RemoveAll()` (same pattern as `UninstallOtelCollector`)
- [ ] 5.4 Support dry-run: print preview and return before executing

## 6. Testing & Validation

- [ ] 6.1 Add unit tests for command reconstruction logic (extracting and re-inserting JVM args)
- [ ] 6.2 Manual validation: run a sample Java app, run `dtwiz install otel-java`, verify traces in Dynatrace, then run `dtwiz uninstall otel-java`
