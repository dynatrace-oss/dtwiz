# Spec Delta

## ADDED Requirements

### Requirement: Exit-on-data sessions report like any other watch session

A watch session that ends because `--exit-on-data` is set SHALL report the same self-monitoring events as a session ended any other way: the `st=snp` event for the poll that found the data, then one `st=com` event. Both events SHALL be delivered before the process exits.

#### Scenario: Session ends on first data

- **GIVEN** the user runs `dtwiz watch --exit-on-data`
- **WHEN** a poll finds data for the first time and the session ends because of it
- **THEN** the system sends one `st=snp` event carrying the time-to-first-data for every signal type that poll found, encoded as defined for snapshot events
- **AND** then sends one `st=com` event with User-Agent `dtwiz/<ver>;c=wch;st=com`
- **AND** both events reach the tenant before the process exits, even though the session ends right after the snapshot

#### Scenario: Session ends at the timeout without data

- **GIVEN** the user runs `dtwiz watch --exit-on-data`
- **AND** no signal type received data
- **WHEN** the session ends at the 10-minute mark
- **THEN** the system sends one `st=com` event and no `st=snp` event, as for any session that ends with no data
