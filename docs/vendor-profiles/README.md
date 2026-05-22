# RTSP Vendor Profiles

Vendor profiles now sit under the broader compatibility profile registry. See
[Compatibility Corpus](../compatibility/servers.md) for the full SSDP, device
XML, M3U, and evidence model.

This directory remains the RTSP wire-behavior policy. `satip-lab` intentionally
ships non-spec RTSP behavior only when it is backed by captured vendor evidence.

## Implemented profiles

| Profile | Status | Documentation |
|---------|--------|---------------|
| `spec` | Implemented | [spec.md](spec.md) |

Unknown profile names fall back to `spec`. Metadata-only compatibility profiles
keep spec-compatible RTSP behavior until trace-backed quirks are documented.

## Evidence requirement

A non-spec profile must include a document in this directory before it is added
to `SATIP_LAB_PROFILE` or the `SATIP_LAB_VENDOR_PROFILE` selector alias. The
document must include:

- Vendor, model, firmware version, and capture date.
- Whether the source is a pcap, RTSP text trace, or manual hardware transcript.
- The exact observed behavior being modeled, such as header casing, session id
  format, required method order, busy status code, and idle timeout behavior.
- At least one sanitized trace excerpt or a link to a repository artifact that
  can be inspected by maintainers.
- Known gaps where the simulator intentionally does less than the hardware.

Invented approximations are out of scope. If a TRIAX, TELestar, Kathrein,
Digital Devices, FRITZ!Box, Inverto, or other profile cannot cite a trace, keep
RTSP behavior on the spec profile plus the generic runtime scenarios until
evidence exists.
