# `spec` Vendor Profile

The `spec` profile is the strict RTSP baseline and a backward-compatible alias
for the original vendor profile. It models the stable SAT>IP-facing behavior
that `satip-lab` has used as its v1 baseline.

## RTSP behavior

| Behavior | `spec` value |
|----------|--------------|
| Session header casing | `Session` |
| Transport header casing | `Transport` |
| Session id format | Numeric, zero-padded to eight digits |
| SETUP Session timeout | `Session: <id>;timeout=60` |
| DESCRIBE before SETUP | Not required |
| Startup `tuner_busy` status | `503 Service Unavailable` |
| Lab tuner exhaustion status | `503 Service Unavailable` |
| No-signal scenario status | `503 Service Unavailable` with `Reason: no signal` |

## Scope

This profile is intentionally strict and deterministic. It is suitable for
client tests that verify the generic SAT>IP control path, M3U import, tuner
allocation, RTP startup, and scenario handling.

It is not an assertion that all SAT>IP hardware behaves this way. Vendor quirks
belong in separate profiles only after trace-backed documentation is available.
