# Supported Simulation Profile

Version: **v1.6-lab** (Go implementation)

This document defines what `satip-lab` guarantees for clients. Anything not listed here is **not simulated**.

For a side-by-side comparison with real SAT>IP hardware behavior, see [support-matrix.md](support-matrix.md).

## Endpoints

| Endpoint | Path / port | Content |
|----------|-------------|---------|
| HTTP | `{PUBLIC_HOST}:{PUBLIC_HTTP_PORT}` | Status, XML, M3U, XMLTV |
| RTSP | `{PUBLIC_HOST}:{PUBLIC_RTSP_PORT}` | Session control |
| SSDP | UDP `1900` | M-SEARCH responses (disable with `SATIP_LAB_SSDP_PORT=0`) |
| Lab API | `/api/config/schema`, `/api/clock`, `/api/schema`, `/api/status`, `/api/catalog`, `/api/muxes`, `/api/services`, `/api/tuners`, `/api/sessions`, `/api/events`, `/api/scenario`, `/api/reset` | JSON lab state and runtime scenarios — see `docs/api.md` |

## Device description

- UPnP root with `deviceType` = `urn:ses-com:device:SatIPServer:1`
- UPnP `specVersion` `1.0` and stable root `configId="1"`
- Fields: `friendlyName`, `manufacturer`, `modelName`, optional `modelNumber`, `UDN`, `presentationURL`
- SAT>IP extensions: `satip:X_SATIPCAP` advertises synthetic DVB-S2 tuner capacity as `DVBS2-{SATIP_LAB_TUNERS}`, and `satip:X_SATIPM3U` points to the active profile's M3U path.
- `SATIP_LAB_PROFILE` changes advertised identity for compatibility tests. Metadata-only profiles do not add undocumented RTSP quirks.

## Channel list (M3U)

By default, five fixed DACH-oriented services are exposed with Astra 19.2E-style tuning params:

| # | Name |
|---|------|
| 1 | Das Erste HD |
| 2 | ZDF HD |
| 3 | arte HD |
| 4 | 3sat HD |
| 5 | phoenix HD |

Each entry includes `tvg-id`, `tvg-name`, `group-title`, and an RTSP URL with `src`, `freq`, `pol`, `msys`, `sr`, `pids`.

Services are grouped into a deterministic lab catalog of muxes. Das Erste HD and arte HD share one mux; the other services use separate muxes. Tuning params are validated against this catalog during RTSP `SETUP`.

Set `SATIP_LAB_CATALOG` or `--catalog` to load a YAML service catalog instead. The bundled `fixtures/astra-19.2e-dach.yaml` fixture contains 25 DACH-oriented services and is also available in the Docker image at `/app/fixtures/astra-19.2e-dach.yaml`. See `docs/catalog.md`.

The default profile serves `/desc.xml` and `/channels.m3u`. Profiles may
advertise another descriptor or M3U path; for example
`SATIP_LAB_PROFILE=tvheadend` advertises and serves `/satip_server/desc.xml`
and `/channellist.m3u`.

## Compatibility profiles

`SATIP_LAB_PROFILE=generic-satip-1.2` is the default profile. Available profiles:

- `generic-satip-1.2`
- `spec`
- `minisatip`
- `tvheadend`
- `triax-tss400`
- `telestar-digibit-r1`
- `kathrein-exip`
- `digital-devices-octopus-net`

Profiles affect SSDP `SERVER`/`ST`/`USN`/`LOCATION`, device XML identity,
advertised M3U path, and RTSP behavior knobs. The public corpus lives under
`docs/compatibility/`. Profiles below `captured-trace` confidence keep
spec-compatible RTSP behavior and use the normal lab RTP/tuner model.

`SATIP_LAB_VENDOR_PROFILE` remains as an RTSP profile selector alias. Prefer `SATIP_LAB_PROFILE` for new configurations.

## XMLTV EPG

- `GET /epg/xmltv.xml` returns deterministic XMLTV for the active catalog.
- XMLTV `channel id` equals the M3U `tvg-id`; XMLTV `display-name` equals the M3U `tvg-name`.
- Timestamps use XMLTV `YYYYMMDDhhmmss +ZZZZ` format in `Europe/Berlin`.
- Default `SATIP_LAB_EPG_CLOCK` is `fixed:2026-03-29T01:30:00+01:00`, so the default 24-hour window crosses the Berlin DST spring-forward transition.
- `GET /api/clock` returns the active EPG clock.
- Schedule density is mixed by service for realistic client list behavior. See `docs/epg.md`.

## In-stream EIT

- Generated synthetic service TS includes minimal DVB EIT present/following sections on PID `0x0012`.
- EIT p/f uses table id `0x4E`, section `0` for present and section `1` for following.
- EIT event names and timing are derived from the same deterministic `SATIP_LAB_EPG_CLOCK` schedule model as XMLTV.
- Runtime `epg_gap` suppresses generated EIT p/f for the targeted service or mux while synthetic media packets continue.
- Runtime `epg_mismatch` remains XMLTV-only; EIT stays bound to the tuned service id.
- Runtime `epg_stale` affects HTTP `Last-Modified` only and does not change TS packets.
- EIT is generated only for synthetic service TS. `SATIP_LAB_TS_PATH` and decodable sample profiles are served unchanged.

## RTSP

The default compatibility profile preserves the stable v1 RTSP behavior:
`Session` and `Transport` header casing, numeric zero-padded session ids,
`Session: <id>;timeout=60` on `SETUP`, `SETUP` allowed without prior `DESCRIBE`,
and `503 Service Unavailable` for tuner-busy behavior. See
[vendor-profiles/spec.md](vendor-profiles/spec.md).

| Method | Behavior |
|--------|-----------|
| OPTIONS | `200`, advertises `OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER` |
| DESCRIBE | `200`, minimal `application/sdp` body for client compatibility checks |
| SETUP | `200`, `Session`, `Transport` with client ports |
| PLAY | `200`, `Session`, starts RTP MPEG-TS to client RTP port; accepts `pids`, `addpids`, and `delpids` control updates |
| PAUSE | `200`, `Session`, stops RTP while keeping the simulated tuner/session allocated |
| TEARDOWN | `200`, `Session`, stops RTP |
| GET_PARAMETER | `200`, `Session`, validates the RTSP session as a keepalive |

- Responses terminated with `\r\n\r\n`.
- CSeq echoed from request.
- `SETUP` uses `Transport: ... destination=IP` when supplied; otherwise it sends RTP to the RTSP TCP peer address.
- RTSP sessions advertise `timeout=60`; valid RTSP requests with a `Session` header refresh activity, and a background reaper expires idle sessions.
- `SETUP` allocates or shares a simulated tuner for the requested mux.
- `pids`, `addpids`, and `delpids` update the lab session PID set for client compatibility. `pids=all` is represented as `pids_all: true` in lab state. Malformed or out-of-range PID values are rejected with `400 Bad Request`; explicit `delpids=<pid>` from all-mode is rejected because exclusions are not modeled. Generated RTP remains the deterministic service payload.
- Unknown tuning/service requests return `404 Not Found`.
- Tuner exhaustion returns `503 Service Unavailable` (lab allocator).

### Scenarios

**Startup (env):**

| `SATIP_LAB_SCENARIO` | Effect |
|----------------------|--------|
| `normal` | Default; lab handles tuner allocation and `503` on exhaustion |
| `tuner_busy` | Every `SETUP` → `503 Service Unavailable` (before lab allocation) |

**Runtime (lab API):** `GET`/`POST /api/scenario`

| Runtime scenario | Effect |
|------------------|--------|
| `normal` | Default lab behavior |
| `no_signal` | Valid `SETUP` → `503` with `Reason: no signal`; no tuner allocated |
| `bad_m3u` | `/channels.m3u` returns malformed playlist content for import/parser tests |
| `rtp_stop` | `PLAY` succeeds, RTP starts, then packet sending stops after a short deterministic burst |
| `slow_rtsp` | RTSP responses are delayed by 250 ms |
| `malformed_psi` | RTP/TS framing remains valid, but generated PAT/PMT table headers are corrupted |
| `rtp_loss` | Every third RTP packet is dropped |
| `rtp_jitter` | Every third RTP packet is delayed by 40 ms |
| `cc_errors` | MPEG-TS continuity counters are corrupted while packet framing is preserved |
| `epg_gap` | XMLTV omits a deterministic programme window for a targeted service or mux |
| `epg_mismatch` | XMLTV uses `zdf-mismatch.invalid` for ZDF HD instead of the M3U `tvg-id` |
| `epg_stale` | XMLTV `Last-Modified` is 48 hours before the lab clock |

See `docs/api.md` for request/response shapes.

## RTP

- Payload type **33** (MP2T)
- Unicast UDP to `client_port` from SETUP
- ~1316 byte TS chunks, ~10 ms pacing
- Runtime `rtp_stop` scenario limits RTP to a short deterministic burst after successful `PLAY`.
- Runtime `malformed_psi` scenario corrupts generated PAT/PMT table headers while keeping RTP and MPEG-TS packet boundaries intact.
- Runtime `rtp_loss`, `rtp_jitter`, and `cc_errors` scenarios apply deterministic RTP/TS impairments for repeatable client tests.
- By default, each service gets distinct generated MPEG-TS packets with PAT/PMT-shaped PSI, minimal EIT p/f, PES-like audio/video payloads, and service-specific markers.
- If `SATIP_LAB_SAMPLE_PROFILE=h264_aac_short`, ZDF HD uses a generated H.264/AAC MPEG-TS test pattern; all other services keep distinct synthetic TS.
- If `SATIP_LAB_SAMPLE_PROFILE=h264_silent`, ZDF HD uses the same style of H.264 test pattern with silent AAC audio for audio-selection and muted-audio behavior tests.
- If `SATIP_LAB_TS_PATH` points to a readable file, that file is looped for every service instead.

## Lab model

- `SATIP_LAB_TUNERS` controls the simulated tuner count.
- Sessions on the same mux share one tuner.
- Sessions on different muxes consume separate tuners.
- `TEARDOWN` releases the session and frees a tuner once no sessions remain on its mux.
- Lab state and active RTP senders are kept in memory and reset on process restart or `POST /api/reset`.
- `POST /api/scenario` changes the runtime lab scenario; unknown names return `400` and leave the scenario unchanged.
- `POST /api/scenario` accepts optional `service_id` and `mux_id` fields for tune-aware RTSP/RTP scenarios and `epg_gap`. HTTP-only, XMLTV-wide, and pre-tune effects remain global.

## Not simulated

- TCP interleaved RTSP
- RECORD
- Tuner signal strength, BER, SNR
- Real DVB scanning or RF signal acquisition
- Real broadcast EPG feeds or full DVB EIT schedule generation
- SAT>IP HTML UI beyond minimal status page
- HTTPS
- Authentication and vendor-specific management APIs
- Non-spec vendor quirks unless backed by a documented captured trace or owned-hardware profile

## Client compatibility intent

Designed for SAT>IP client tests such as:

- SSDP discovery and device description fetch
- M3U import and RTSP URL parsing (tuning query parameters)
- XMLTV EPG import, channel mapping, Now/Next behavior, DST handling, and stale-data diagnostics
- DVB EIT present/following parser fallback against generated synthetic TS
- RTSP session setup and teardown
- Tuner pool exhaustion and same-mux sharing (via lab + RTSP)
- RTP MPEG-TS playback (distinct synthetic TS per service, one decodable ZDF HD sample profile, or one file via `SATIP_LAB_TS_PATH`)
- Lab observability (`GET /api/status`, `/api/tuners`, `/api/events`)

**Not** a substitute for validation against real SAT>IP hardware or full servers such as minisatip.
