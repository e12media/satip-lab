# SAT>IP Hardware Support Matrix

This matrix compares `satip-lab` v1 lab behavior with behavior a client may see from real SAT>IP hardware or full SAT>IP servers.

Status legend:

| Status | Meaning |
|--------|---------|
| Supported | Implemented as a stable v1 simulator contract. |
| Partial | Implemented enough for client development, with documented simplifications. |
| Lab-only | Intentional simulator feature that real hardware usually does not provide. |
| Not simulated | Out of scope for v1; validate against real hardware or a full server. |

## Discovery and description

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| SSDP M-SEARCH discovery | Replies to SAT>IP device search over multicast. | Replies for `urn:ses-com:device:SatIPServer:1`; SSDP can be disabled with `SATIP_LAB_SSDP_PORT=0`. | Supported | Test discovery locally; in CI, prefer manual host configuration because multicast is often unavailable. |
| UPnP device description | Serves a SAT>IP device XML with device identity and SAT>IP extensions. | Serves `/desc.xml` with stable identity, `configId`, `X_SATIPCAP`, and `X_SATIPM3U`. | Supported | Assert parser compatibility and advertised tuner count. |
| Presentation web UI | Many devices provide a vendor-specific status or configuration UI. | Minimal status page only. | Partial | Do not depend on UI behavior in automated client tests. |
| Vendor metadata quirks | Varies by manufacturer and firmware. | `SATIP_LAB_PROFILE` can advertise generic SAT>IP, minisatip, TVHeadend, TRIAX, TELestar, Kathrein, or Digital Devices metadata. Non-spec RTSP quirks still require trace-backed documentation before implementation. | Partial | Use profiles for parser and discovery hardening; validate undocumented protocol quirks and RTP timing against hardware until trace-backed profiles exist. |

## Channel and catalog behavior

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| M3U channel list | Some devices or companion servers expose an M3U list; contents vary. | Serves five stable DACH services by default, or a configured YAML catalog, with SAT>IP RTSP URLs and Astra-style tuning parameters. | Supported | Use the default list for smoke tests and `SATIP_LAB_CATALOG` for larger import/UI tests. |
| XMLTV EPG | Full servers often expose EPG through XMLTV or server-specific APIs; SAT>IP hardware may not. | Serves deterministic XMLTV for the active catalog with Berlin DST coverage and EPG error scenarios. | Lab-only | Use for client EPG parser, tvg-id mapping, Now/Next, stale-data, and mismatch tests. |
| DVB EIT present/following | Broadcast TS often carries EIT p/f in DVB SI. | Generated synthetic TS carries minimal EIT p/f on PID `0x0012` derived from the lab clock. Sample media profiles and `SATIP_LAB_TS_PATH` are not rewritten. | Partial | Use for EIT fallback parser tests; validate full DVB SI behavior against real streams. |
| Stable service catalog | Real hardware usually exposes tuners, not a fixed curated channel catalog. | Provides deterministic mux/service data through M3U, YAML catalog fixtures, and lab JSON APIs. | Lab-only | Use service IDs and mux IDs to write repeatable allocation and scenario tests. |
| Real DVB scanning | Hardware tunes RF and scans DVB tables. | No RF scanning. Catalog is static. | Not simulated | Validate scan workflows with real hardware or full TV server fixtures. |
| Dynamic channel updates | Real environments may change with scans, playlists, or server configuration. | Catalog is fixed at process start. | Not simulated | Use separate fixtures for client behavior around channel churn. |

## RTSP session control

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| OPTIONS | Advertises supported RTSP methods. | Advertises `OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER`. | Supported | Assert method negotiation and CSeq handling. |
| DESCRIBE | Some clients request SDP before playback. | Returns a minimal `application/sdp` response. | Partial | Test client compatibility checks, not detailed SDP semantics. |
| SETUP | Allocates tuner resources for a requested transponder/service and client UDP ports. | Validates catalog tuning parameters, allocates or shares simulated mux tuners, returns `Session` and `Transport`. | Supported | Test successful setup, same-mux sharing, invalid tuning, and tuner exhaustion. |
| PLAY | Starts RTP MPEG-TS delivery. | Starts UDP RTP payload type 33 and supports PID updates through `pids`, `addpids`, and `delpids`. | Supported | Test playback startup, PID control requests, and RTP packet reception. |
| PAUSE | May stop streaming while keeping a session. | Stops RTP while keeping the simulated RTSP session and tuner allocation. | Supported | Test pause/resume client state machines. |
| GET_PARAMETER keepalive | Commonly used to keep sessions alive. | Validates the session and refreshes timeout activity. | Supported | Test keepalive scheduling and recovery from idle timeout. |
| TEARDOWN | Releases stream resources. | Stops RTP and releases the lab session/tuner when appropriate. | Supported | Test cleanup and subsequent tuner reuse. |
| TCP interleaved RTSP/RTP | Supported by some servers and clients, often as a firewall/NAT fallback. | Accepts `RTP/AVP/TCP;interleaved=<rtp>-<rtcp>` SETUP and sends `$`-framed RTP payload type 33 over the RTSP TCP connection. | Supported | Test client TCP fallback, frame-boundary parsing, and cleanup on PAUSE/TEARDOWN. |
| RECORD | Not part of the current lab target. | Not implemented. | Not simulated | Use other tools for recording workflows. |
| Authentication and HTTPS | Vendor/server dependent. | Not implemented. | Not simulated | Test auth and TLS against separate fixtures. |

## RTP and MPEG-TS media

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| RTP MPEG-TS over UDP | Sends MPEG-TS packets over RTP payload type 33. | Sends unicast UDP RTP with approximately 1316-byte TS payload chunks and deterministic pacing. | Supported | Test RTP socket setup, packet reception, timestamps, and sequence tracking. |
| Real audio/video elementary streams | Carries broadcast audio/video that media decoders can play. | Synthetic TS remains transport-focused. `SATIP_LAB_SAMPLE_PROFILE=h264_aac_short` makes ZDF HD carry a generated H.264/AAC test pattern, and `h264_silent` uses silent AAC. A configured `SATIP_LAB_TS_PATH` still loops one file for all services. | Partial | Use synthetic TS for routing tests, the ZDF HD sample profile for decoder smoke tests, and `SATIP_LAB_TS_PATH` for custom fixtures. |
| DVB PSI/SI fidelity | Hardware emits real broadcast PSI/SI tables. | Generates PAT/PMT-shaped markers and minimal EIT p/f sufficient for lab differentiation, plus a malformed PSI scenario. | Partial | Test parser robustness around presence or corruption markers, not full DVB SI correctness. |
| Network impairments | Real networks may drop, delay, or reorder packets unpredictably. | Deterministic `rtp_loss`, `rtp_jitter`, `cc_errors`, and `rtp_stop` scenarios. | Lab-only | Use to exercise client recovery paths repeatably in CI. |
| Signal metrics | Hardware may expose lock, signal strength, SNR, BER, and similar RF state. | Exposes deterministic synthetic frontend telemetry through `/api/tuners` and `/api/status`, with `signal_degraded`, `lock_loss`, `slow_lock`, and timeline recovery states. | Partial | Use for signal-quality UI, retry, and diagnostics tests; validate real RF measurement behavior against hardware. |

## Tuner and resource model

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| Fixed tuner capacity | Hardware has a finite tuner count and advertises capability. | `SATIP_LAB_TUNERS` controls simulated DVB-S2 tuner capacity and `X_SATIPCAP` advertising. | Supported | Test UI/resource behavior across one or many tuners. |
| Same-transponder sharing | Multiple services on one transponder can share RF tuning resources on real systems. | Sessions on the same lab mux share one simulated tuner. | Supported | Test client behavior for concurrent same-mux playback. |
| Cross-transponder exhaustion | Tuning too many distinct muxes can exhaust hardware. | Sessions on different lab muxes consume separate simulated tuners and return `503` when exhausted. | Supported | Test busy handling and retry/backoff logic. |
| RF lock acquisition timing | Real devices may have variable tune and lock delays. | Normal `SETUP` allocates a tuner in `state=tuning` and reports `state=locked` after deterministic `lock_ms=250`; timeline recovery from `lock_loss` reports `state=recovering` for the same window. `slow_rtsp` adds a fixed RTSP delay, and `slow_lock` reports deterministic frontend `lock_ms=1200` while setup/play still succeed. | Partial | Test timeout thresholds with `slow_rtsp`, normal tune/lock lifecycle, and UI/status behavior with `slow_lock`; validate RF timing against hardware. |
| Conditional access and scrambled services | Some broadcasts require CAM/card handling. | Not implemented. | Not simulated | Use hardware/full TV server environments for scrambled-service workflows. |

## Lab API and scenarios

| Capability | Real SAT>IP hardware expectation | `satip-lab` v1 behavior | Status | Client test guidance |
|------------|----------------------------------|--------------------------|--------|----------------------|
| Runtime status API | Hardware/server APIs are vendor specific. | Stable JSON lab API for status, catalog, tuners, sessions, events, scenarios, reset, and schemas. | Lab-only | Use for assertions in automated tests without packet sniffing. |
| Hardware-style status surface | Real devices often expose uptime, firmware/model identity, active streams, tuner/frontend state, and counters through vendor-specific pages or APIs. | `/api/status` includes a deterministic lab-only `hardware` block, and the minimal HTML page renders the same hardware-like state. | Lab-only | Use for client management/status UI tests; validate vendor management APIs against real hardware. |
| Runtime error forcing | Real hardware failures are hard to trigger deterministically. | Runtime scenarios force no signal, tuner busy, malformed M3U, malformed PSI, RTP stop/loss/jitter, continuity errors, slow RTSP, and RF-like frontend telemetry variants. Startup `tuner_busy` and normal allocator exhaustion also cover busy-tuner paths using the active compatibility profile's busy status. | Lab-only | Drive negative-path tests directly from CI. |
| Scenario timelines | Real hardware failures evolve over time but are hard to reproduce exactly. | `POST /api/scenario` can install deterministic elapsed-millisecond timelines and `GET /api/scenario` reports the active step. | Lab-only | Use to test client recovery flows where signal, transport, or EPG conditions change during a test. |
| Scenario targeting | Real hardware failures may affect a tuner, mux, service, cable, or network path. | Tune-aware scenarios can target a service, a mux, or their intersection where context exists. | Lab-only | Verify clients handle mixed healthy/unhealthy channel sets. |
| Reset to known state | Real hardware reset is disruptive and slow. | `POST /api/reset` clears in-memory lab sessions and tuner state. | Lab-only | Reset between test cases to keep suites deterministic. |

## Compatibility tiers

Use these tiers when deciding where a client test belongs:

| Tier | Use `satip-lab`? | Purpose |
|------|------------------|---------|
| Parser and discovery tests | Yes | Device XML, M3U, SAT>IP URL parsing, SSDP where multicast is available. |
| RTSP state-machine tests | Yes | Session setup, play, pause, keepalive, teardown, timeout, PID updates. |
| Resource and error-path tests | Yes | Tuner exhaustion, no signal, bad playlists, packet impairments, reset. |
| Decoder/player quality tests | Sometimes | Use the ZDF HD H.264/AAC sample profile for CI smoke tests, or a custom TS fixture through `SATIP_LAB_TS_PATH` when you need exact media. |
| RF, scanning, vendor, auth, and TLS tests | No | Validate with real hardware, minisatip, TVHeadend, or specialized fixtures. |
