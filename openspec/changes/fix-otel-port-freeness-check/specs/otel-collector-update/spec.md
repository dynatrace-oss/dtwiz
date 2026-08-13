## REMOVED Requirements

### Requirement: Generated config ports must not conflict with already-running collectors

**Reason**: This requirement described port-conflict detection for generating a new collector config, but that
behavior belongs to `install otel`'s config generation, not to this capability's config-patching flow, which never
generates a new config or allocates ports. The requirement was also incomplete on its own terms: it only covered the
Prometheus metrics port's `localhost` binding and missed that the `otlp`/`health_check` receivers bind `0.0.0.0`,
which does not conflict with a `localhost`-only probe on most systems.

**Migration**: See the `otel-collector-port-allocation` capability, which covers this behavior where it actually
lives, with the missed case corrected.
