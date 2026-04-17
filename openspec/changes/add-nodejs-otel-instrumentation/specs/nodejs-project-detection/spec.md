# Node.js Project Detection

## ADDED Requirements

### Requirement: Node.js project discovery

The system SHALL detect Node.js projects by scanning for `package.json` files across CWD and common development directories, using the existing `scanProjectDirs()` infrastructure.

#### Scenario: Standard Node.js project detected

- **GIVEN** a directory contains a `package.json` file
- **WHEN** the project scanner runs
- **THEN** the directory is included as a `ScannedProject` with `package.json` as a marker

#### Scenario: node_modules directories excluded

- **GIVEN** a `package.json` exists inside a `node_modules/` subdirectory
- **WHEN** the project scanner runs
- **THEN** that `package.json` is NOT treated as a project root

#### Scenario: No Node.js projects found

- **GIVEN** no `package.json` files exist in CWD or scanned directories
- **WHEN** `DetectNodePlan` runs
- **THEN** it returns nil and logs a debug message

### Requirement: Monorepo detection

The system SHALL detect monorepos by checking for a `"workspaces"` field in `package.json`. When present, workspace directories containing their own `package.json` SHALL be listed as individual `ScannedProject` entries.

#### Scenario: Monorepo with workspaces array

- **GIVEN** a `package.json` contains `"workspaces": ["packages/*"]`
- **AND** `packages/api/package.json` and `packages/web/package.json` exist
- **WHEN** the project scanner runs
- **THEN** both `packages/api` and `packages/web` appear as individual projects alongside the monorepo root

#### Scenario: Monorepo with workspaces object

- **GIVEN** a `package.json` contains `"workspaces": {"packages": ["packages/*"]}`
- **AND** workspace child directories contain `package.json`
- **WHEN** the project scanner runs
- **THEN** the child directories appear as individual projects

#### Scenario: No workspaces field

- **GIVEN** a `package.json` does not contain a `"workspaces"` field
- **WHEN** the project scanner runs
- **THEN** only the single project root is listed (no workspace expansion)

### Requirement: Next.js project identification

The system SHALL identify Next.js projects by checking for `next.config.js`, `next.config.ts`, `next.config.mjs`, or `next` in `package.json` dependencies/devDependencies.

#### Scenario: Next.js detected via config file

- **GIVEN** a project directory contains `next.config.js` (or `.ts` or `.mjs`)
- **WHEN** `isNextJSProject()` is called
- **THEN** it returns true

#### Scenario: Next.js detected via package dependency

- **GIVEN** a project's `package.json` lists `"next"` in `dependencies` or `devDependencies`
- **AND** no `next.config.*` file exists
- **WHEN** `isNextJSProject()` is called
- **THEN** it returns true

#### Scenario: Regular Node.js project (not Next.js)

- **GIVEN** a project has no `next.config.*` and no `next` in dependencies
- **WHEN** `isNextJSProject()` is called
- **THEN** it returns false

### Requirement: Nuxt project identification

The system SHALL identify Nuxt projects by checking for `nuxt.config.js`, `nuxt.config.ts`, `nuxt.config.mjs`, `nuxt.config.mts`, or `nuxt` in `package.json` dependencies/devDependencies.

#### Scenario: Nuxt detected via config file

- **GIVEN** a project directory contains `nuxt.config.ts` (or `.js`, `.mjs`, or `.mts`)
- **WHEN** `detectNodeFramework()` is called
- **THEN** it returns `"nuxt"`

#### Scenario: Nuxt detected via package dependency

- **GIVEN** a project's `package.json` lists `"nuxt"` in `dependencies` or `devDependencies`
- **AND** no `nuxt.config.*` file exists
- **WHEN** `detectNodeFramework()` is called
- **THEN** it returns `"nuxt"`

#### Scenario: Next.js takes precedence over Nuxt

- **GIVEN** a project has both `next` and `nuxt` in dependencies
- **WHEN** `detectNodeFramework()` is called
- **THEN** it returns `"next"` (Next.js checked first)

#### Scenario: Regular Node.js project (not Nuxt)

- **GIVEN** a project has no `nuxt.config.*` and no `nuxt` in dependencies
- **AND** the project is not a Next.js project
- **WHEN** `detectNodeFramework()` is called
- **THEN** it returns `""`

### Requirement: Package manager detection from lockfiles

The system SHALL auto-detect the project's package manager by checking for lockfiles.

#### Scenario: npm detected

- **GIVEN** a project contains `package-lock.json`
- **WHEN** `detectNodePackageManager()` is called
- **THEN** it returns `"npm"`

#### Scenario: yarn detected

- **GIVEN** a project contains `yarn.lock`
- **WHEN** `detectNodePackageManager()` is called
- **THEN** it returns `"yarn"`

#### Scenario: pnpm detected

- **GIVEN** a project contains `pnpm-lock.yaml`
- **WHEN** `detectNodePackageManager()` is called
- **THEN** it returns `"pnpm"`

#### Scenario: No lockfile found

- **GIVEN** a project contains no recognized lockfile
- **WHEN** `detectNodePackageManager()` is called
- **THEN** it returns `"npm"` as the default

### Requirement: Running Node.js process detection

The system SHALL detect running Node.js processes and correlate them to project directories, reusing the existing `detectProcesses()` and `matchProcessesToProjects()` infrastructure.

#### Scenario: Running node process matched to project

- **GIVEN** a `node` process is running with a working directory inside a detected project
- **WHEN** `matchProcessesToProjects()` runs
- **THEN** the project's `RunningProcessIDs` includes that process's PID

#### Scenario: npm processes excluded from detection

- **GIVEN** processes matching `npm` are running
- **WHEN** `detectNodeProcesses()` filters processes
- **THEN** npm management processes are excluded (following the existing pattern)

### Requirement: Node entrypoint detection

The system SHALL detect the project entrypoint from `package.json` fields or common filenames. For Next.js projects, a special marker entrypoint is returned.

#### Scenario: Entrypoint from package.json main field

- **GIVEN** `package.json` contains `"main": "server.js"` and the file exists
- **WHEN** `detectNodeEntrypoints()` is called
- **THEN** it returns `["server.js"]`

#### Scenario: Entrypoint from scripts.start

- **GIVEN** `package.json` contains `"scripts": {"start": "node app.js"}` and `app.js` exists
- **WHEN** `detectNodeEntrypoints()` is called
- **THEN** it returns `["app.js"]`

#### Scenario: Entrypoint fallback to common filenames

- **GIVEN** `package.json` has no `main` or `scripts.start`, but `index.js` exists
- **WHEN** `detectNodeEntrypoints()` is called
- **THEN** it returns `["index.js"]`

#### Scenario: TypeScript variants detected

- **GIVEN** only `app.ts` exists (no `.js` files)
- **WHEN** `detectNodeEntrypoints()` scans fallback names
- **THEN** it detects `app.ts`

#### Scenario: Next.js project entrypoint

- **GIVEN** the project is identified as Next.js
- **WHEN** `detectNodeEntrypoints()` is called
- **THEN** it returns a Next.js marker entrypoint indicating `next start` should be used

#### Scenario: Nuxt project entrypoint

- **GIVEN** the project is identified as Nuxt
- **WHEN** `detectNodeEntrypoints()` is called
- **THEN** it returns a Nuxt marker entrypoint indicating `nuxt start` should be used
