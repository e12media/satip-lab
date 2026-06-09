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

## v1.7 Behavioral Profile Evidence Pipeline

- [x] `satip-lab-smoke --json` emits machine-readable RTSP/RTP evidence for profile review.
- [x] `satip-lab-smoke --profile <name>` records the intended compatibility profile in JSON evidence.
- [x] Checked-in compatibility profile YAML is validated by Go tests.
- [x] Optional behavior evidence fields document trace-backed RTSP/session/timing observations.
- [x] Non-spec behavior evidence is rejected unless profile confidence is `captured-trace` or `owned-hardware`.
- [x] Runtime profile loading remains Go-defined; YAML behavior promotion is a later reviewed step.

## v1.8 RF/Tuner Telemetry V1

- [x] `/api/tuners` and `/api/status` expose deterministic frontend telemetry for each tuner.
- [x] Normal tuned frontends report synthetic signal strength, SNR, BER/PER, lock timing, and last lock transition.
- [x] `signal_degraded`, `lock_loss`, and `slow_lock` scenarios provide deterministic RF-like status variants.
- [x] RF telemetry scenarios support `service_id` and `mux_id` targeting.
- [x] RTSP setup/play behavior remains unchanged for telemetry-only scenarios.

## v1.9 Scenario Timelines

- [x] `POST /api/scenario` accepts deterministic scenario timelines with elapsed millisecond steps.
- [x] `GET /api/scenario` exposes active timeline state, step index, elapsed time, and configured steps.
- [x] Timeline transitions record lab events.
- [x] Timeline steps reuse existing scenario validation, targeting, and duration rules.
- [x] Posting a normal scenario object clears an active timeline.

## v1.10 TCP Interleaved RTSP/RTP

- [x] `SETUP` accepts `RTP/AVP/TCP;interleaved=<rtp>-<rtcp>` transport requests.
- [x] `PLAY` sends RTP payload type 33 inside RTSP `$` interleaved frames on the TCP connection.
- [x] `PAUSE`, `TEARDOWN`, and session timeout stop interleaved streaming without closing the RTSP session unexpectedly.
- [x] UDP RTP behavior remains unchanged.

## v1.11 Hardware-Style Management/Status Surface

- [x] `/api/status` remains backward compatible and adds a nested lab-only `hardware` block.
- [x] Hardware status includes uptime, profile-aware identity metadata, active stream counts, tuner counts, frontend lock counts, and simple network counters.
- [x] The minimal HTML status page exposes the same hardware-like state for humans.
- [x] `/api/agent/context` advertises `hardware_status`.
- [x] API schema version `1.7` documents the hardware status models.

## v1.12 Frontend/Tuner Lifecycle V2

- [x] Frontend telemetry models deterministic lifecycle states: `idle`, `tuning`, `locked`, `degraded`, `lost`, and `recovering`.
- [x] `SETUP` allocates a tuner in `state=tuning`; normal status reports `state=locked` after a deterministic 250 ms lock window.
- [x] Same-mux session sharing preserves the shared frontend lifecycle instead of restarting lock acquisition.
- [x] Scenario timelines can drive lock loss and recovery without changing default RTSP success behavior.
- [x] `/api/agent/context` advertises `frontend_lifecycle`.

## v1.13 Hardware Fault Scenarios V1

- [x] `cold_boot` adds deterministic RTSP startup latency.
- [x] `tuner_wedged` rejects SETUP until `POST /api/reset` clears the wedged state.
- [x] `rtp_blackhole` drops all RTP packets while leaving the RTSP session alive.
- [x] `delayed_psi` delays startup PAT/PMT evidence before normal RTP resumes.
- [x] `signal_recovery` exposes recovering-to-locked frontend telemetry for missing-signal recovery flows.
- [x] Fault scenarios are documented in docs and `/api/agent/context`.

## Digital Twin Roadmap

Use a spine-first implementation order: land the current foundations, add
hardware-like observability, then add time-varying faults and evidence-backed
device personalities. Non-spec vendor behavior remains blocked until sanitized
trace or owned-hardware evidence exists.

| Priority | Work item | GitHub tracking | Implementation notes |
|----------|-----------|-----------------|----------------------|
| P0 | Land current foundation PRs | Issues #4, #5, #6; PRs #3, #13, #19, #15 | Merge in dependency-safe order after maintainer review: behavioral evidence, RF telemetry, scenario timelines, then TCP interleaved RTSP/RTP. Reconcile schema/version docs after merges. |
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
