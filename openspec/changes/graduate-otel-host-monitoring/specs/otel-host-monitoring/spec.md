## REMOVED Requirements

### Requirement: Host monitoring is gated behind the experimental flag until fully implemented and tested

**Reason**: Host monitoring is released and is now part of the default `install otel` behavior.

**Migration**: Use the existing host-monitoring requirements as the default behavior. `--experimental` and `DTWIZ_EXPERIMENTAL=true` are no longer required for host-monitoring collector configuration or privilege notices.
