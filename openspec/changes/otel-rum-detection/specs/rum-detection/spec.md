# Spec: RUM Detection

## NEW Requirements

### Requirement: Scan project directory for RUM injection eligibility

The system SHALL scan the project directory selected during `dtwiz install otel` (falling back to `--project` or the current directory when no project was selected) and determine whether Real User Monitoring can be set up via automatic HTML injection or requires manual tag placement.

#### Scenario: Framework config file detected

- **GIVEN** the project directory contains a config file associated with a JavaScript framework (Next.js, Nuxt)
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode (because frameworks manage their build output and prevent direct HTML file modification)
- **AND** the reason identifies the detected framework by name

#### Scenario: Next.js project with a root layout file

- **GIVEN** the project is a Next.js project
- **AND** a root layout file exists as `app/layout`, `src/app/layout`, `pages/_document` or `src/pages/_document` with extension `.tsx`, `.jsx`, `.ts` or `.js`
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode
- **AND** the result identifies that layout file as the place to add the tag
- **AND** App Router layouts take priority over Pages Router documents when both exist

#### Scenario: Next.js project without a recognizable layout file

- **GIVEN** the project is a Next.js project
- **AND** none of the standard root layout files exist
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode
- **AND** the result does not identify a file to add the tag to

#### Scenario: Malformed package.json

- **GIVEN** the project directory contains a `package.json` that cannot be parsed as valid JSON
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the system treats `package.json` as not indicating any framework
- **AND** continues to the HTML file scan

#### Scenario: index.html found, no framework

- **GIVEN** the project directory contains a writable `index.html` file alongside other `.html` files
- **AND** no framework config file or Create React App dependency is present
- **WHEN** the RUM detection scan runs
- **THEN** the system returns auto injection mode (because `index.html` is the app entry point and can be safely modified)
- **AND** the result includes only the `index.html` path(s), not other `.html` files

#### Scenario: Static HTML files found without index.html, no framework

- **GIVEN** the project directory contains one or more writable `.html` files, none named `index.html`
- **AND** no framework config file or Create React App dependency is present
- **WHEN** the RUM detection scan runs
- **THEN** the system returns auto injection mode (because all found files are likely page-level HTML)
- **AND** the result includes the full paths of all writable HTML files, sorted

#### Scenario: HTML files exist only inside build output directories

- **GIVEN** the project directory contains `.html` files only inside directories such as `dist/`, `build/`, `out/`, `.next/`, `.nuxt/`, `.output/`, or `__pycache__/`
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the system excludes those files from the injectable file list (because they are build output, not source files)
- **AND** returns manual injection mode with reason indicating no writable HTML files were found

#### Scenario: Deeply nested index.html

- **GIVEN** the project directory contains `index.html` only at a deeply nested path (e.g. `src/main/resources/templates/index.html`)
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the system returns auto injection mode
- **AND** the result includes that `index.html` path

#### Scenario: HTML files outside the selected project directory

- **GIVEN** `.html` files exist in directories outside the selected project directory
- **WHEN** the RUM detection scan runs
- **THEN** those files are never scanned or listed (because only the selected project directory belongs to the application being onboarded)

#### Scenario: Manual injection preview does not name a framework

- **GIVEN** RUM detection returns manual injection mode
- **WHEN** the install preview is shown
- **THEN** the preview states that auto-injection is not possible
- **AND** it does not name the detected framework or other reason
- **AND** it shows the file to add the tag to when one was identified

#### Scenario: HTML files exist inside hidden directories

- **GIVEN** the project directory contains `.html` files only inside hidden directories (names starting with `.`)
- **WHEN** the RUM detection scan runs
- **THEN** those files are excluded from the injectable file list (because hidden directories hold tool caches and configuration, not application source)
- **AND** the system returns manual injection mode with reason indicating no writable HTML files were found

#### Scenario: HTML files exist inside node_modules or .git

- **GIVEN** the project directory contains `.html` files only inside `node_modules/` or `.git/`
- **WHEN** the RUM detection scan runs
- **THEN** those files are excluded and do not appear in the injectable file list (because they are dependencies or version control artifacts, not application source)
- **AND** the system returns manual injection mode with reason indicating no writable HTML files were found

#### Scenario: No HTML files found anywhere

- **GIVEN** the project directory contains no `.html` files outside excluded directories
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode (because there are no injectable HTML files to modify)
- **AND** the reason states that no writable HTML files were found

#### Scenario: HTML file exists but is not writable

- **GIVEN** the project directory contains a `.html` file that the process does not have write permission to
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the non-writable file is excluded from the injectable file list (because it cannot be modified)
- **AND** if no other writable HTML files remain, the system returns manual injection mode

#### Scenario: Framework config and HTML files both present

- **GIVEN** the project directory contains both a framework config file and writable `.html` files
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode (because framework detection takes precedence — the framework will manage HTML, so static files are not injectable)
- **AND** the injectable file list is empty

#### Scenario: Angular-style project with index.html and component templates

- **GIVEN** the project directory contains `index.html` alongside many `.html` component templates (e.g. `app.component.html`, `header.component.html`)
- **AND** no framework config file is present
- **WHEN** the RUM detection scan runs
- **THEN** the system returns auto injection mode
- **AND** only `index.html` is included in the result (component templates are excluded because `index.html` is present)
