# OTel Host Monitoring

## REMOVED Requirements

### Requirement: Host monitoring is gated behind the experimental flag until fully implemented and tested

**Reason**: Host monitoring is released and is now part of the regular `install otel` behavior.

**Migration**: Use the existing host-monitoring requirements as the regular behavior. `--experimental` and `DTWIZ_EXPERIMENTAL=true` are no longer required, and the collector template no longer carries a host-monitoring on/off condition.
