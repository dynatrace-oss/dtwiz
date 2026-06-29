# Tasks

## 1. Implementation

- [x] 1.1 Add `runtime.GOOS == "windows"` branch in `installHelm()` in `pkg/installer/kubernetes.go` that delegates to `installHelmWindows()`
- [x] 1.2 Implement `installHelmWindows()` in `pkg/installer/kubernetes.go`: check winget on PATH via `exec.LookPath("winget")`; if found run `winget install --id Helm.Helm -e --source winget` via `RunCommand`; if winget missing or command fails, return error with manual instructions (`winget install --id Helm.Helm` + `https://helm.sh/docs/intro/install/`)
- [x] 1.3 Add `"runtime"` to imports in `pkg/installer/kubernetes.go`

## 2. Tests

- [x] 2.1 Add unit tests in `pkg/installer/kubernetes_test.go` covering: winget found + succeeds, winget found + fails, winget not found — assert correct error messages per spec scenarios
- [x] 2.2 Verify `make build` succeeds (cross-platform compile: `GOOS=windows go build ./...`)
- [x] 2.3 Run `make lint` — no new lint issues
