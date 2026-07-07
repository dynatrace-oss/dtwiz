## Context

`watchIngest` in `pkg/installer/ingest_watch.go` renders a footer after every install. Currently it always links to QuickStart. Cloud installs (AWS, GCP, Azure) should link to the Clouds app instead, as QuickStart won't show cloud resources at GA.

## Goals / Non-Goals

**Goals:**
- Cloud installs show the Clouds app footer.
- Non-cloud installs keep the QuickStart footer unchanged.

**Non-Goals:**
- Changing any other part of the watch screen.
- Changing footer behavior for OneAgent, OTel, Docker, or Kubernetes.

## Decisions

**Add `cloudInstall bool` to internal `watchIngest`, expose `WatchIngestCloud` publicly.**

The internal function already carries an `awsAccountID` string, which implicitly signals AWS. Rather than reusing that as a placeholder for GCP/Azure (which would trigger AWS-specific DQL queries), a dedicated bool cleanly separates the concern. A new exported `WatchIngestCloud` function lets GCP and Azure call without duplicating logic.

Alternatives considered:
- Reuse `awsAccountID` as placeholder — rejected: would incorrectly run AWS DQL queries for GCP/Azure.
- Pass footer URL/label as strings — rejected: over-engineered for a binary choice.

## Risks / Trade-offs

- Minimal risk — pure rendering change, no API or data model impact.
- If a future cloud method is added, callers must remember to use `WatchIngestCloud`. Mitigated: function name is self-documenting.
