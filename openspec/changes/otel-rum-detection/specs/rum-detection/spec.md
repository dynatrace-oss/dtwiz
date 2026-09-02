# Spec: RUM Detection

## NEW Requirements

### Requirement: Scan project directory for RUM injection eligibility

The system SHALL scan the current project directory when `dtwiz install otel` runs and determine whether Real User Monitoring can be set up via automatic HTML injection or requires manual tag placement.

#### Scenario: Framework config file detected

- **GIVEN** the project directory contains a config file associated with a JavaScript framework (Next.js, Nuxt)
- **WHEN** the RUM detection scan runs
- **THEN** the system returns manual injection mode (because frameworks manage their build output and prevent direct HTML file modification)
- **AND** the reason identifies the detected framework by name

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
