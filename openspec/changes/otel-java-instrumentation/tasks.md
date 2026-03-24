## 1. Pre-flight Validation

- [ ] 1.1 Extend `detectJava()` in `otel_java.go` to be a proper validation gate: check `java` in PATH, parse version from `java -version`, fail if < Java 8
- [ ] 1.2 Call validation at the start of `InstallOtelJava()` — exit with clear error if checks fail
- [ ] 1.3 Add unit tests for version parsing and validation logic

## 2. Agent JAR Download

- [ ] 2.1 Implement `downloadJavaAgent()` function: resolve latest release from `https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest` (follow redirect to get version), download `opentelemetry-javaagent.jar` to `~/opentelemetry/java/`
- [ ] 2.2 Handle existing JAR: check if `~/opentelemetry/java/opentelemetry-javaagent.jar` exists, prompt user to re-download or use existing
- [ ] 2.3 Handle download failure: print clear error with manual download URL
- [ ] 2.4 Add unit tests for release URL resolution logic

## 3. Java Process Detection

- [ ] 3.1 Implement `detectJavaProcesses()`: use `ps ax` to find running `java` processes, parse PID, main class/JAR, arguments, and working directory
- [ ] 3.2 If `jps` is available in PATH, use it as supplementary source for main class names
- [ ] 3.3 Present detected processes as a numbered selection menu with PID, main class/JAR, and working directory
- [ ] 3.4 Handle no-processes case: inform user, offer to provide a manual command to instrument
- [ ] 3.5 Add unit tests for process parsing logic

## 4. Instrumented Launch

- [ ] 4.1 Implement `instrumentJavaProcess()`: parse the original command from `ps`, insert `-javaagent:~/opentelemetry/java/opentelemetry-javaagent.jar` as a JVM flag, set OTEL_* env vars
- [ ] 4.2 Show preview: print the full instrumented command and env vars, prompt `Apply? [Y/n]`
- [ ] 4.3 On confirm: stop the original process (SIGINT), launch the instrumented command as a background detached process
- [ ] 4.4 After launch: verify traces/services appear in Dynatrace via DQL query (reuse existing verification pattern from Python/Collector)
- [ ] 4.5 In dry-run mode: print the instrumented command and env vars without executing

## 5. Uninstall Command

- [ ] 5.1 Add `otel-java` subcommand to `uninstallCmd` in `cmd/uninstall.go`
- [ ] 5.2 Create `pkg/installer/otel_java_uninstall.go` with `UninstallOtelJava(dryRun bool) error`
- [ ] 5.3 Implement: find processes with `-javaagent:.*opentelemetry-javaagent.jar` in their command, list them + JAR directory, prompt for confirmation, stop processes, remove `~/opentelemetry/java/`
- [ ] 5.4 Support dry-run mode

## 6. Testing & Validation

- [ ] 6.1 Add unit tests for command reconstruction logic
- [ ] 6.2 Manual validation: run a sample Java app, install otel-java, verify traces in Dynatrace, then uninstall
