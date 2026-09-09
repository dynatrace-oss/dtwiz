# Tasks: Create and Store Frontend ID

## 1. Project Config Package (`pkg/config/`)

- [x] 1.1 Create `pkg/config/config.go` with `ProjectConfig` struct (map of env URL to `FrontendConfig`), `FrontendConfig` struct, and `Load(projectDir string)` / `Save(projectDir string, cfg *ProjectConfig)` functions
- [x] 1.2 Implement `Load`: read `{projectDir}/.dtwiz/config.yaml`; return empty config (no error) if file does not exist; return error on YAML parse failure
- [x] 1.3 Implement `Save`: create `{projectDir}/.dtwiz/` directory if missing, write config as YAML with `0o600` file permissions
- [x] 1.4 Write unit tests in `pkg/config/config_test.go` covering: file not found returns empty config, valid file is parsed correctly, malformed YAML returns error, save creates directory and file, save overwrites existing file, two environment keys are stored and retrieved independently, updating one environment's entry preserves the other

## 2. RUM Frontend Package (`pkg/installer/otel/internal/rum/`)

- [x] 2.1 Create `pkg/installer/otel/internal/rum/frontend.go` with a `FrontendName(projectDir string) (string, error)` function: resolves absolute path, takes `filepath.Base`, sanitizes (lowercase, non-alphanumeric → hyphen, trim leading/trailing hyphens), appends `-` + first 8 hex chars of `sha256` of the absolute path, prefixes `dtwiz-`, truncates to 255 chars
- [x] 2.2 Write unit tests in `pkg/installer/otel/internal/rum/frontend_test.go` covering: same path produces same name, different paths with same basename produce different names, name starts with `dtwiz-`, special characters in dirname are sanitized, result does not exceed 255 chars

## 3. RUM Frontend API Client (`pkg/installer/otel/internal/rum/frontend.go`)

- [x] 3.1 Add unexported request/response types: `frontendRequest` (`displayName`, `frontendName`, `type`, `enableRum`) and `frontendResponse` (`id`, `frontendName`, `displayName`, `type`) with JSON tags
- [x] 3.2 Implement unexported `createFrontend(ctx context.Context, platform *client.PlatformClient, projectDir string) (frontendResponse, error)`: builds the request, POST to `/platform/rum/v1/frontends`, decodes 201 response, returns error with API message on non-201

## 4. `EnsureFrontend` Function (`pkg/installer/otel/internal/rum/frontend.go`)

- [x] 4.1 Implement `EnsureFrontendApplication(ctx context.Context, platform *client.PlatformClient, envURL, projectDir string) (string, error)`: load project config, return stored ID if present for `envURL`, otherwise call `createFrontend`, save returned ID to config, return ID
- [x] 4.2 Write unit tests for `EnsureFrontendApplication` in `pkg/installer/otel/internal/rum/frontend_test.go` covering: existing ID returned without API call, missing ID triggers API call and ID is saved, API error leaves config unchanged

## 5. Validation

- [x] 5.1 Run `make test` and confirm all new tests pass
- [x] 5.2 Run `make lint` and fix any reported issues
