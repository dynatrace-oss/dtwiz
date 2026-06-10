# Extensions Package — CRUD Operations

## CHANGED Requirements

### Requirement: pkg/extensions exposes Platform API wrappers for Extensions v2

The `pkg/extensions` package SHALL provide reusable functions that call the Dynatrace
Extensions v2 API via a `*client.PlatformClient` (Bearer auth, `.apps.` domain). No
caller outside this package interacts with the raw HTTP layer for extension operations.

The base path for all endpoints is `/platform/extensions/v2/extensions/{extensionName}`.

#### Scenario: InstallExtension activates an extension version

- **GIVEN** a `*client.PlatformClient` and an extension name and version
- **WHEN** `InstallExtension(c, extensionName, version, silent)` is called
- **THEN** it POSTs `{"extensionName": extensionName, "version": version}` to
  `/platform/extensions/v2/extensions/{extensionName}`
- **AND** returns nil on HTTP 200, 201, or 202

#### Scenario: InstallExtension with silent=true ignores 400 and 409

- **GIVEN** `silent` is `true`
- **WHEN** the API returns HTTP 400 (bad request) or 409 (conflict)
- **THEN** `InstallExtension` returns nil without error
- **AND** no error is propagated to the caller (idempotent installs)

#### Scenario: InstallExtension with silent=true still fails on server errors

- **GIVEN** `silent` is `true`
- **WHEN** the API returns HTTP 500 or any other non-2xx status besides 400/409
- **THEN** `InstallExtension` returns an error containing the HTTP status code

#### Scenario: ListMonitoringConfigs returns all items across pages

- **GIVEN** a `*client.PlatformClient` and an extension name
- **WHEN** `ListMonitoringConfigs(c, extensionName)` is called
- **THEN** it GETs `/platform/extensions/v2/extensions/{extensionName}/monitoring-configurations`
- **AND** follows `nextPageKey` pagination until the field is empty
- **AND** returns the concatenated list of all `MonitoringConfigItem` entries

#### Scenario: ListMonitoringConfigs returns error on non-200

- **GIVEN** the API returns a non-200 status
- **WHEN** `ListMonitoringConfigs` processes the response
- **THEN** it returns an error containing the HTTP status code

#### Scenario: CreateMonitoringConfigs returns the first created objectId

- **GIVEN** a `*client.PlatformClient`, an extension name, and a payload slice
- **WHEN** `CreateMonitoringConfigs(c, extensionName, payload)` is called
- **THEN** it POSTs the payload to the monitoring-configurations endpoint
- **AND** returns the `objectId` from the first result entry with a 200 or 201 inner code

#### Scenario: CreateMonitoringConfigs returns error on non-2xx

- **GIVEN** the API returns a non-2xx status (not 200, 201, or 207)
- **WHEN** `CreateMonitoringConfigs` processes the response
- **THEN** it returns an error containing the HTTP status code and up to 400 bytes of the response body

#### Scenario: DeleteMonitoringConfig deletes by objectId

- **GIVEN** a `*client.PlatformClient`, an extension name, and an objectId
- **WHEN** `DeleteMonitoringConfig(c, extensionName, objectID)` is called
- **THEN** it DELETEs `/platform/extensions/v2/extensions/{extensionName}/monitoring-configurations/{objectID}`
- **AND** returns nil on HTTP 200 or 204

#### Scenario: DeleteMonitoringConfig returns error on non-2xx

- **GIVEN** the API returns a status other than 200 or 204
- **WHEN** `DeleteMonitoringConfig` processes the response
- **THEN** it returns an error containing the HTTP status code and response body

#### Scenario: ListInstalledVersions returns installed versions of an extension

- **GIVEN** a `*client.PlatformClient` and an extension name
- **WHEN** `ListInstalledVersions(c, extensionName)` is called
- **THEN** it GETs `/platform/extensions/v2/extensions/{extensionName}`
- **AND** returns the `items` array (each with a `version` field) on HTTP 200
- **AND** returns an error containing the HTTP status code on any non-200 response

#### Scenario: GetLatestInstalledVersion returns the highest dotted-numeric version

- **GIVEN** the tenant has multiple installed versions of an extension (e.g. `1.0.11`, `1.0.10`, `1.2.0`)
- **WHEN** `GetLatestInstalledVersion(c, extensionName)` is called
- **THEN** it returns the highest version compared segment-wise as integers
- **AND** non-numeric segments are treated as 0 for comparison
- **AND** returns an error when no installed versions are found

#### Scenario: GetMonitoringConfig fetches a single configuration by objectId

- **GIVEN** a `*client.PlatformClient`, an extension name, and an objectId
- **WHEN** `GetMonitoringConfig(c, extensionName, objectID)` is called
- **THEN** it GETs `/platform/extensions/v2/extensions/{extensionName}/monitoring-configurations/{objectID}`
- **AND** returns a `*MonitoringConfig` containing `scope` and a non-nil `value` map on HTTP 200
- **AND** returns an error containing the HTTP status code and up to 400 bytes of body on any non-200 response

#### Scenario: UpdateMonitoringConfig replaces an existing configuration via PUT

- **GIVEN** a `*client.PlatformClient`, an extension name, an objectId, and a `*MonitoringConfig`
- **WHEN** `UpdateMonitoringConfig(c, extensionName, objectID, cfg)` is called
- **THEN** it PUTs `{"scope": cfg.Scope, "value": cfg.Value}` to
  `/platform/extensions/v2/extensions/{extensionName}/monitoring-configurations/{objectID}`
- **AND** returns nil on HTTP 200, 201, or 204
- **AND** returns an error containing the HTTP status code and up to 400 bytes of body on any other status
