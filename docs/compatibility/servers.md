# SAT>IP Compatibility Corpus

This corpus tracks SAT>IP server models that `satip-lab` can imitate at the
lab-protocol level. It is intended for compatibility hardening, not for claiming
hardware equivalence.

## Confidence Levels

| Level | Meaning |
|-------|---------|
| `spec` | Baseline SAT>IP behavior implemented by `satip-lab`. |
| `public-doc` | Public manuals, source, issue threads, or support docs identify model metadata or documented paths. |
| `user-report` | A user supplied diagnostics from a real installation, but no raw trace is available. |
| `captured-trace` | Sanitized `desc.xml`, SSDP headers, M3U, RTSP transcript, or pcap backs the modeled behavior. |
| `owned-hardware` | Maintainers can rerun the behavior against hardware or a full server under test. |

Only `captured-trace` and `owned-hardware` evidence should introduce non-spec
wire quirks such as unusual RTSP status codes, header casing, method order
requirements, session formatting, PID update behavior, or RTP timing faults.
Profiles with weaker evidence may advertise model metadata and documented M3U
paths, but must keep spec-compatible RTSP behavior.

## Built-In Profiles

| Profile | Runtime status | Confidence | Notes |
|---------|----------------|------------|-------|
| `generic-satip-1.2` | Implemented | `spec` | Default compatibility profile; equivalent to the existing simulator behavior. |
| `spec` | Implemented | `spec` | Backward-compatible alias for the original RTSP vendor profile. |
| `minisatip` | Implemented metadata profile | `public-doc` | Advertises minisatip identity; RTSP behavior remains spec-compatible until traces document quirks. |
| `tvheadend` | Implemented metadata/profile paths | `public-doc` | Advertises TVHeadend identity, `/satip_server/desc.xml`, and `/channellist.m3u`, which are documented in Tvheadend public examples. |
| `triax-tss400` | Implemented metadata profile | `public-doc` | Model metadata only; protocol quirks require trace promotion. |
| `telestar-digibit-r1` | Implemented metadata profile | `public-doc` | Model metadata only; satip-axe or hardware traces should promote behavior later. |
| `kathrein-exip` | Implemented metadata profile | `public-doc` | Model metadata only; protocol quirks require trace promotion. |
| `digital-devices-octopus-net` | Implemented metadata/profile path | `public-doc` | Advertises Octopus NET identity and `/octoserve/octonet.xml`, matching public support docs for descriptor loading. |

## Runtime Usage

```bash
SATIP_LAB_PROFILE=tvheadend docker compose up
SATIP_LAB_PROFILE=minisatip docker compose up
SATIP_LAB_PROFILE=telestar-digibit-r1 docker compose up
```

`SATIP_LAB_PROFILE` controls SSDP headers, SSDP `LOCATION`, device XML metadata,
advertised M3U path, and RTSP profile knobs. `SATIP_LAB_VENDOR_PROFILE` maps
through the same profile registry as an RTSP profile selector alias.

## Adding Evidence

Add one YAML file under `docs/compatibility/profiles/` per profile. Keep raw
captures sanitized and small enough for review, or link to a repository artifact.
Each entry should include:

- `model`
- `source`
- `confidence`
- `desc_xml`
- `ssdp_headers`
- `m3u_sample`
- `rtsp_transcript`
- `known_quirks`
- `supported_methods`
- `tuner_behavior`
- `playback_notes`

When evidence upgrades a profile to `captured-trace` or `owned-hardware`, update
the matching runtime profile and tests in the same PR.
