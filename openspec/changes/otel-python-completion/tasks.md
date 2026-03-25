## 1. Pre-flight Validation

- [ ] 1.1 Add a `validatePythonPrerequisites()` function in `otel_python.go` that checks: `exec.LookPath("python3")` (or fall back to `"python"` + version check like existing `detectPython()`), `exec.LookPath("pip3")` or `exec.LookPath("pip")`, and `exec.Command(python, "-m", "venv", "--help").Run()` for venv module
- [ ] 1.2 Return `fmt.Errorf` with clear user-facing messages on failure (no `os.Exit` — errors bubble to cmd layer). For venv: suggest `apt install python3-venv` on Debian/Ubuntu
- [ ] 1.3 Call `validatePythonPrerequisites()` at the start of `InstallOtelPython()` — return error before any detection work
- [ ] 1.4 Add unit tests for the validation function with various missing-prerequisite scenarios

## 2. Testing

- [ ] 2.1 Add unit tests for the validation function with various missing-prerequisite scenarios
- [ ] 2.2 Manual validation: install otel-python on a system missing python3/pip/venv and verify clear error messages are shown
