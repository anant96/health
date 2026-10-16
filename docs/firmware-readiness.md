# Firmware readiness example

This optional example evaluates whether an infrastructure-maintenance operation can proceed based on observed device firmware. All device IDs and versions used here are synthetic.

The caller supplies an Inspector, a logical ResourceID, an execution AttemptID, a Side (source or destination), a set of Devices and the immutable policy snapshot applicable to that side. A policy's Revision identifies its snapshot. Different sides and policy revisions may produce different decisions for the same firmware observations.

An evaluation inspects each requested device through health.Check callbacks and uses health.Checker for execution. Successful callbacks supply firmware versions, while failed or absent observations remain incomplete. The checker is constructed with WithDisabledCache so repeated calls to Evaluate perform fresh inspection.

The operational decision and the completeness of supporting evidence are distinct concepts:
- All observations are present and none is excluded: Allowed=true, Definitive=true.
- Some observations are missing and none is excluded: Allowed=true, Definitive=false.
- Any observed version is excluded: Allowed=false, Definitive=true, including when other devices are uninspectable.

These rules are intentionally enforced by the uncached evaluator rather than by the aggregate UP/DOWN/UNKNOWN health status. This module does not implement a reusable verdict cache.

Run the package tests using `go test ./validation`, and use `go test -race ./validation` to check race instrumentation. The repository's regular Go dependencies must be available for offline execution.
