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

## v1.7 RF/Tuner Telemetry V1

- [x] `/api/tuners` and `/api/status` expose deterministic frontend telemetry for each tuner.
- [x] Normal tuned frontends report locked state, synthetic signal strength, SNR, BER/PER, lock timing, and last lock transition.
- [x] `signal_degraded`, `lock_loss`, and `slow_lock` scenarios provide deterministic RF-like status variants.
- [x] RF telemetry scenarios support `service_id` and `mux_id` targeting.
- [x] RTSP setup/play behavior remains unchanged for telemetry-only scenarios.

## v1.8 Scenario Timelines

- [x] `POST /api/scenario` accepts deterministic scenario timelines with elapsed millisecond steps.
- [x] `GET /api/scenario` exposes active timeline state, step index, elapsed time, and configured steps.
- [x] Timeline transitions record lab events.
- [x] Timeline steps reuse existing scenario validation, targeting, and duration rules.
- [x] Posting a normal scenario object clears an active timeline.

## Next High-Value Slices

- Promote metadata-only profiles to trace-backed non-spec behavior after sanitized traces or pcaps are documented.
- Developer ergonomics: SSE event stream, `satip-lab-smoke --json`, multi-server discovery.

## Non-goals

- Real DVB scanning.
- Real broadcast EPG ingestion.
- Full DVB EIT/SI fidelity.
- Recording or timeshift.
- Authentication and user management.
- Replacing TVHeadend, minisatip, or real hardware validation.
