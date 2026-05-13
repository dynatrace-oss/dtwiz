# Development

## Table of Contents

- [Folder structure](#folder-structure)
- [Best practices](#best-practices)
  - [Cross-platform first](#cross-platform-first)
  - [Error handling](#error-handling)
  - [Never shell out unnecessarily](#never-shell-out-unnecessarily)
  - [Input validation](#input-validation)
- [Feature flags](#feature-flags)

## Folder structure

Before writing code, please familiarize yourself with how our project is laid out. Adding new functionality in the wrong layer is a common source of review feedback.

```text
main.go                         # entry point → cmd.Execute()
cmd/
  root.go                       # cobra root, persistent flags: --environment, --access-token, --platform-token
  auth.go                       # getDtEnvironment(), accessToken(), platformToken(), URL helpers
  analyze.go, recommend.go, setup.go, status.go, version.go
  install.go, update.go, uninstall.go
pkg/
  analyzer/                     # system detection (platform, Docker, K8s, OneAgent, OTel, AWS, Azure, services)
  recommender/                  # recommendation engine (priority-ranked, method-based)
  installer/                    # shared utilities (URL/token helpers, RunCommand) + per-method installers
    installer.go                # AuthHeader(), APIURL(), AppsURL(), ExtractTenantID(), RunCommand()
    oneagent.go, kubernetes.go, docker.go, otel.go, otel_update.go, otel_python.go, aws.go
    dynakube.tmpl, otel.tmpl, aws.tmpl   # embedded Go templates
    sudo_unix.go, sudo_windows.go
scripts/
  install.sh, install.ps1       # curl|sh installer scripts
```

Don't add new top-level packages without discussing it with a maintainer first.

## Best practices

### Cross-platform first

dtwiz runs on Linux, macOS, and Windows. Every change must work on all three.

Always use `filepath.Join` for constructing file paths — never string concatenation:

```go
// correct
venvDir := filepath.Join(proj.Path, ".venv")

// wrong
venvDir := proj.Path + "/.venv"
```

For logic that really diverges between platforms, split it into build-tagged files rather than using `runtime.GOOS` checks inline:

```go
// sudo_unix.go
//go:build !windows

func needsSudo() bool { return os.Getuid() != 0 }
```

```go
// sudo_windows.go
//go:build windows

func needsSudo() bool { return false }
```

When a `runtime.GOOS` check is unavoidable inside shared code, handle all relevant platforms explicitly rather than relying on an implicit else. Unix-only operations like `chmod` must be guarded:

```go
if runtime.GOOS != "windows" {
    os.Chmod(scriptPath, 0755)
}
```

### Error handling

Wrap errors with context using `%w` so the full call chain is visible:

```go
return fmt.Errorf("downloading OneAgent installer: %w", err)
```

Include platform information on failures where it helps diagnosis:

```go
return fmt.Errorf("failed to start process on %s/%s: %w", runtime.GOOS, runtime.GOARCH, err)
```

Distinguish user errors (bad config, missing environment variable) from system errors (network, permissions) — they need different messages and, in some cases, different exit handling.

### Never shell out unnecessarily

Prefer Go stdlib or libraries over spawning a subprocess. When a subprocess is necessary, use `RunCommand` / `RunCommandQuiet` from `pkg/installer/installer.go` — they handle stdout/stderr streaming and error wrapping consistently.

### Input validation

Validate all user-supplied values (environment URLs, file paths, service names) at the boundary — in the cobra command handler — before passing them into packages. Packages can trust their inputs are valid.

## Feature flags

Feature flags let us ship incomplete or experimental functionality behind a gate so `main` stays releasable at all times. Users can opt into early behavior via an environment variable without any code changes on their side.

All flags are defined in [`pkg/featureflags/featureflags.go`](../../pkg/featureflags/featureflags.go). Adding a new one takes three steps.

**1. Declare the flag constant:**

```go
const (
    AllRuntimes Flag = iota
    MyNewFlag        // add here
)
```

**2. Register it in the `registry` slice:**

```go
{
    MyNewFlag,
    "my-new-flag",        // kebab-case — becomes the --my-new-flag CLI flag
    "DTWIZ_MY_NEW_FLAG",  // environment variable users can set
    false,                // default value
    "short description shown in --help",
    false,                // bound: internal boolean variable binding for cobra — always false at init, populated at parse time
},
```

**3. Use it at the call site:**

```go
if featureflags.IsEnabled(featureflags.MyNewFlag) {
    // experimental path
}
```

The resolution order of values for any feature flag is: CLI flag `>` environment variable (`"true"` or `"1"`) `>` default.

Register the flag with the [cobra](https://github.com/spf13/cobra) library on the relevant command using `featureflags.RegisterFlags(flags)` and call `featureflags.ApplyCLIOverrides(flags)` in the CLI command's `PreRun` hook definition.
