# Spec: setup-recommendations

## ADDED Requirements

### Requirement: The recommendation list is introduced by a signal lead-in

Both the interactive menu and the non-interactive recommendation listing SHALL print a single lead-in immediately below the section divider and above the entries, naming the signal types every recommendation ingests: logs, metrics, and traces. The signal types SHALL NOT be repeated inside individual entry titles.

#### Scenario: Interactive menu rendered

- **GIVEN** at least one actionable recommendation exists
- **WHEN** `dtwiz setup` renders the recommendations section
- **THEN** a lead-in naming logs, metrics, and traces is printed between the divider and the first entry

#### Scenario: Non-interactive listing rendered

- **GIVEN** at least one recommendation exists
- **WHEN** `dtwiz recommend` renders its list
- **THEN** the same lead-in is printed between the divider and the first entry

#### Scenario: Entry titles omit signal types

- **GIVEN** the lead-in has been printed
- **WHEN** any recommendation entry is rendered
- **THEN** its title describes what is monitored and by which method, and does not restate logs, metrics, or traces

---

### Requirement: Host-scoped recommendation titles state scope and method

Every recommendation that monitors the local host SHALL be titled with a shared scope phrase naming the host and its services, followed by a parenthetical naming the method that distinguishes it from the others. Two host-scoped entries SHALL NOT be distinguishable only by a trailing method name appended to an otherwise unrelated phrase, and no host-scoped title SHALL describe its scope as a machine's services.

#### Scenario: New collector option

- **GIVEN** the recommendation to deploy a new OpenTelemetry Collector is produced
- **WHEN** it is rendered in either the menu or the listing
- **THEN** its title names the host and its services, and identifies the method as a new OpenTelemetry Collector

#### Scenario: Existing collector option

- **GIVEN** an OpenTelemetry Collector is already running on the host
- **WHEN** the resulting recommendation is rendered
- **THEN** its title names the host and its services, and identifies the method as the existing OpenTelemetry Collector

#### Scenario: OneAgent option

- **GIVEN** the recommendation to install OneAgent on the host is produced
- **WHEN** it is rendered
- **THEN** its title names the host and its services, and identifies the method as OneAgent

#### Scenario: All three host-scoped options offered together

- **GIVEN** a collector is running, the platform supports OneAgent, and experimental recommendations are enabled
- **WHEN** the menu is rendered
- **THEN** all three entries share the same scope phrase and differ only in the method named in their parenthetical

---

### Requirement: Recommendation identity is independent of title text

Selection and dispatch SHALL key on a recommendation's method identifier rather than its title. Changing title wording SHALL NOT change which installer a selection invokes, nor the set or order of recommendations produced.

#### Scenario: Selecting a retitled entry

- **GIVEN** a recommendation's title wording has changed
- **WHEN** the user selects that entry in the menu
- **THEN** the same installer runs as before the wording changed

#### Scenario: Recommendation set unchanged

- **GIVEN** the same analyzed system
- **WHEN** recommendations are generated
- **THEN** the same methods are produced in the same order as before the wording changed
