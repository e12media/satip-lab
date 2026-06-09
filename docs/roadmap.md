# Roadmap

`satip-lab` is moving toward a deterministic SAT>IP lab server: realistic SAT>IP-facing behavior, synthetic media and tuner state, and CI-friendly observability.

## v0.2 Lab Core

Status: **complete enough for integration review**.

- [x] Catalog-backed mux/service model.
- [x] Configurable simulated tuner count.
- [x] Tuner allocation, same-mux sharing, and release on `TEARDOWN`.
- [x] Distinct generated MPEG-TS payloads per service when no TS file is configured.
- [x] Generated TS is packet-aligned and carries PAT/PMT-style service markers plus service-specific audio/video packets.
- [x] JSON status API for tuners, sessions, events, and reset.
- [x] JSON catalog API for catalog, muxes, and services.
- [x] Integration tests cover tuner exhaustion, release, status visibility, and distinct service RTP payloads.

Deferred from v0.2:

- Full DVB PSI/SI table fidelity.
- Valid elementary audio/video decoding.
- Custom external catalog files.

## v0.3 Variant and Error Forcing

- [x] Runtime scenario switching (`GET`/`POST /api/scenario`).
- [x] `no_signal` scenario for deterministic RTSP `SETUP` failure without tuner allocation.
- [x] Named scenarios for RTP stop and bad M3U.
- [x] Named scenarios for slow RTSP and malformed PSI.
- [x] RTP packet loss, jitter, and continuity counter errors.
- [x] Per-service or per-mux scenario targeting.

## v0.4 SAT>IP Compatibility Surface

- [x] `DESCRIBE`, `PAUSE`, and `GET_PARAMETER`.
- [x] Session timeout behavior.
- [x] PID update support for `addpids` and `delpids` where clients rely on it.
- [x] More complete SAT>IP device capability advertising.

## v1.0 Stable Lab Contract

- [x] Versioned configuration schema.
- [x] Stable API schema.
- [x] Published Docker image under `ghcr.io/e12media/satip-lab`.
- [x] Anonymous GHCR pull verified after package visibility is set to public.
- [x] Multi-arch image manifest for `linux/amd64` and `linux/arm64`.
- [x] CI recipes for common client stacks.
- [x] Clear support matrix against real SAT>IP hardware behavior.

## v1.1 EPG XMLTV Foundation

- [x] Deterministic `/epg/xmltv.xml` endpoint for bundled services.
- [x] Fixed default `Europe/Berlin` lab clock crossing the 2026 spring-forward DST window.
- [x] `GET /api/clock` for client-side assertions.
- [x] XMLTV channel ids aligned with M3U `tvg-id` values and display names aligned with M3U `tvg-name`.
- [x] Mixed schedule density across services.
- [x] `epg_gap`, `epg_mismatch`, and `epg_stale` runtime scenarios.

## v1.2 Decodable Media Fixture

- [x] Built-in `SATIP_LAB_SAMPLE_PROFILE` selector.
- [x] `h264_aac_short` profile: ZDF HD uses a short generated H.264/AAC MPEG-TS test pattern.
- [x] `h264_silent` profile: ZDF HD uses a generated H.264 test pattern with silent AAC audio.
- [x] Other services remain distinct synthetic TS under sample profiles.
- [x] `SATIP_LAB_TS_PATH` remains the highest-priority global override and loops one file for every service.
- [x] Docker image includes the generated sample-profile assets and defaults to `h264_aac_short`.

## v1.3 Vendor Profile Framework

- [x] `SATIP_LAB_VENDOR_PROFILE` configuration and CLI flag.
- [x] `spec` profile as the strict default RTSP behavior contract.
- [x] RTSP responses route header casing, setup timeout formatting, and busy status through the active profile.
- [x] Documentation policy requiring trace-backed evidence before adding non-spec vendor quirks.

## v1.4 File-Backed Catalogs

- [x] `SATIP_LAB_CATALOG` configuration and `--catalog` CLI flag.
- [x] YAML catalog loader with startup validation and field-specific errors.
- [x] `/channels.m3u`, `/epg/xmltv.xml`, `/api/catalog`, RTSP service matching, and tuner sharing all use the active catalog.
- [x] Bundled 25-service `fixtures/astra-19.2e-dach.yaml` DACH/Astra fixture.
- [x] Docker image includes the bundled fixture at `/app/fixtures/astra-19.2e-dach.yaml`.
- [x] Catalog format documentation for client CI and UI scale tests.

## v1.5 EIT Present/Following

- [x] Minimal DVB EIT p/f sections on PID `0x0012` for generated synthetic TS.
- [x] EIT timing and titles derived from the deterministic EPG lab clock.
- [x] `epg_gap` suppresses generated EIT for targeted services or muxes.
- [x] Explicit TS files and decodable sample profiles remain unchanged.
- [x] TS-level tests cover EIT table id, section numbers, event timing, duration, and descriptors.

## v1.6 Compatibility Corpus + Lab Profiles

- [x] `SATIP_LAB_PROFILE` configuration and `--profile` CLI flag.
- [x] Built-in compatibility profiles for generic SAT>IP, minisatip, TVHeadend, TRIAX TSS 400, TELestar DIGIBIT R1, Kathrein EXIP, and Digital Devices Octopus NET.
- [x] Profile-specific SSDP `SERVER`/`ST`/`USN`/`LOCATION`, device XML identity/path, and advertised M3U path.
- [x] TVHeadend profile advertises and serves `/channellist.m3u`.
- [x] Public compatibility corpus under `docs/compatibility/` with confidence levels and YAML profile records.
- [x] `SATIP_LAB_VENDOR_PROFILE` remains as an RTSP profile selector alias.

## Digital Twin Roadmap

Use a spine-first implementation order: land the current foundations, add
hardware-like observability, then add time-varying faults and evidence-backed
device personalities. Non-spec vendor behavior remains blocked until sanitized
trace or owned-hardware evidence exists.

| Priority | Work item | GitHub tracking | Implementation notes |
|----------|-----------|-----------------|----------------------|
| P0 | Land current foundation PRs | Issues #4, #5, #6; PRs #3, #13, #14, #15 | Merge in dependency-safe order after maintainer review: behavioral evidence, RF telemetry, scenario timelines, then TCP interleaved RTSP/RTP. Reconcile schema/version docs after merges. |
| P1 | Hardware-style management/status surface | #11 | Add lab-only hardware-style uptime, firmware/profile metadata, active streams, frontend state, and network counters while keeping `/api/status` backward compatible. |
| P2 | Frontend/tuner lifecycle V2 | #16 | Model deterministic frontend states such as idle, tuning, locked, degraded, lost, and recovering using RF telemetry and scenario timeline hooks. |
| P3 | Hardware fault scenarios V1 | #17 | Add explicit deterministic faults such as cold boot delay, wedged tuner until reset, RTP dies while RTSP remains alive, delayed first PAT/PMT, and missing-signal recovery. |
| P4 | Capture-backed profile ingestion | #9 | Add tooling to validate sanitized RTSP trace or pcap-derived summaries and prepare reviewed profile evidence without runtime YAML behavior loading. |
| P5 | Trace-backed profile promotion and personality profiles | #7, #12 | Promote one real profile only after evidence exists; implement only documented observed behavior and keep generic/spec behavior stable. |
| P6 | DVB SI fidelity V1 | #8 | Add bounded fixture-driven SI realism such as SDT/NIT basics, PMT descriptors, PCR timing markers, scrambled flags, or multiple audio descriptors. |
| P7 | Multi-server and topology simulation | #10 | Support deterministic multiple advertised identities, distinct tuner pools, duplicate names, stale LOCATIONs, and CI guidance for multicast-limited environments. |

Each tracked issue should carry acceptance criteria, implementation notes, a
test plan, and blocked-by references. Keep P5 issues open until real
trace-backed or owned-hardware evidence is available.

## Next High-Value Slices

- Follow the Digital Twin Roadmap above.
- Developer ergonomics after the current spine: SSE event stream, richer smoke artifacts, and topology fixtures.

## Non-goals

- Real DVB scanning.
- Real broadcast EPG ingestion.
- Full DVB EIT/SI fidelity.
- Recording or timeshift.
- Authentication and user management.
- Replacing TVHeadend, minisatip, or real hardware validation.
