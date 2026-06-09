# Coding Agent Guide

Use this guide when a coding agent needs to integrate, test, or debug a SAT>IP client with `satip-lab`.

## Start Here

1. Start `satip-lab` locally or as a CI service container.
2. Poll `GET /api/agent/context` until it returns `200 OK`.
3. Configure the client under test from the returned `test_env` values:
   - `SATIP_TEST_HTTP_URL`
   - `SATIP_TEST_RTSP_URL`
4. Reset the simulator between independent tests with `POST /api/reset`.
5. Use `POST /api/scenario` to switch deterministic failure modes.

The simulator is a lab server, not a production SAT>IP implementation. Keep client tests scoped to the supported profile in `docs/supported-profile.md`.

## Repository Change Workflow

When changing this repository, use the default delivery workflow:

1. Create or switch to a `codex/` branch before editing.
2. Implement the smallest scoped change and add tests for behavior changes.
3. Run `make test` and `make lint`.
4. If runtime behavior, Docker, CI, media generation, or advertised lab contracts changed, run `make docker-up`, `make smoke`, and `make docker-down`.
5. Open a PR with verification evidence and client-facing compatibility notes.
6. Request or spawn a PR review pass, implement confirmed review issues, then rerun relevant tests.
7. Publish containers and merge only when explicitly requested by a maintainer or through the release workflow.

Documentation-only changes still use a branch and PR; container smoke is optional unless the docs change Docker, CI, release, or client runtime instructions.

## Runtime Discovery

```bash
curl -fsS http://127.0.0.1:8875/api/agent/context
```

The response includes:

- Advertised HTTP and RTSP URLs.
- Self-contained EPG evidence URLs, including `urls.xmltv` and `urls.clock`.
- Ready-to-use client test environment variables.
- Catalog source, catalog size, bundled fixture path, and a sample RTSP tune URL.
- Feature flags for custom catalogs, compatibility evidence tooling, compatibility profiles, XMLTV, EIT present/following, frontend lifecycle, frontend telemetry, hardware-style status, RTSP interleaved TCP, RTSP/RTP smoke, and runtime scenarios.
- Runtime profile name from `runtime.profile`.
- Compatibility profile names and corpus path from `compatibility`.
- Runtime scenario names and whether they can be scoped by `service_id` or `mux_id`.
- Documentation paths agents should read before editing this repo.

The `features` object describes capabilities supported by the simulator binary. Some capabilities are mode-dependent at runtime; for example, EIT present/following is generated for synthetic TS, while `SATIP_LAB_TS_PATH` and decodable sample profiles are served unchanged.

When `features.frontend_lifecycle` is true, client tests can assert that normal
`SETUP` reports `frontend.state=tuning` before the deterministic lock window
elapses, then `frontend.state=locked`. Timeline recovery from `lock_loss` may
report `frontend.state=recovering` before returning to locked.

## Recommended Client Test Flow

```bash
export SATIP_TEST_HTTP_URL=http://127.0.0.1:8875
export SATIP_TEST_RTSP_URL=rtsp://127.0.0.1:554/

curl -fsS "$SATIP_TEST_HTTP_URL/api/agent/context" >/dev/null
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/reset" >/dev/null
curl -fsS "$SATIP_TEST_HTTP_URL/channels.m3u" | grep -q "ZDF HD"
curl -fsS "$SATIP_TEST_HTTP_URL/epg/xmltv.xml" | grep -q "zdf.de"
```

After basic HTTP checks, run the client project's own RTSP/RTP integration tests against `SATIP_TEST_RTSP_URL`.
Clients that support TCP fallback can request `RTP/AVP/TCP;unicast;interleaved=0-1`
in SETUP and assert `$`-framed RTP payload type 33 on the RTSP TCP connection.
For guide clients that parse in-stream DVB data, include a test for synthetic EIT present/following on PID `0x0012` when using generated TS.

## Catalog-Aware Tests

The default catalog has five stable DACH services. For larger import, guide, tuner, and UI tests, use the bundled 25-service fixture:

```bash
SATIP_LAB_CATALOG=fixtures/astra-19.2e-dach.yaml go run ./cmd/satip-lab
```

In Docker, use `/app/fixtures/astra-19.2e-dach.yaml`. See `docs/catalog.md`.

## Compatibility Profile Tests

Use `SATIP_LAB_PROFILE` to make the simulator advertise different SAT>IP server
identities while keeping deterministic lab behavior:

```bash
SATIP_LAB_PROFILE=tvheadend go run ./cmd/satip-lab
SATIP_LAB_PROFILE=minisatip go run ./cmd/satip-lab
SATIP_LAB_PROFILE=telestar-digibit-r1 go run ./cmd/satip-lab
```

Profiles affect SSDP headers and `LOCATION`, device XML identity/path, advertised M3U path, and
trace-backed RTSP knobs. Metadata-only profiles intentionally keep spec-compatible
RTSP behavior. See `docs/compatibility/servers.md`.

## Scenario Recipes

| Scenario | Use it to test |
|----------|----------------|
| `bad_m3u` | Playlist parser rejection and user-facing import errors. |
| `no_signal` | Tune failures after valid RTSP `SETUP`. |
| `slow_rtsp` | Client timeout and retry behavior. |
| `cold_boot` | Startup latency; expect every RTSP response to be delayed by 750 ms. |
| `tuner_busy` | Tuner exhaustion handling without needing pre-filled sessions. |
| `tuner_wedged` | Wedged frontend handling; expect 503 setup failures until `POST /api/reset`. |
| `rtp_stop` | Playback loss after successful `PLAY`; expect exactly 3 RTP packets before delivery stops. |
| `rtp_blackhole` | RTP receive timeout while the RTSP session remains alive. |
| `rtp_loss` | RTP packet loss tolerance; expect every third RTP packet to be dropped. |
| `rtp_jitter` | Buffering and timing behavior; expect every third RTP packet to be delayed by 40 ms. |
| `delayed_psi` | Startup parser tolerance; expect a deterministic gap before first PAT/PMT evidence arrives. |
| `cc_errors` | MPEG-TS continuity-counter error handling. |
| `malformed_psi` | PAT/PMT validation and error reporting. |
| `signal_degraded` | Signal-quality UI and retry handling; expect `/api/tuners` frontend `state=degraded`. |
| `lock_loss` | Lost-lock UI and recovery handling; expect `/api/tuners` frontend `state=lost`. |
| `signal_recovery` | Missing-signal recovery UI; expect `/api/tuners` frontend `state=recovering` before locked. |
| `slow_lock` | Slow-lock UI and timeout tolerance; expect `/api/tuners` frontend `state=tuning` and `lock_ms=1200`. |
| `epg_gap` | Missing schedule windows. |
| `epg_mismatch` | M3U/XMLTV channel id mismatch handling. |
| `epg_stale` | Stale EPG refresh behavior. |

Example:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"name":"rtp_loss","service_id":"zdf-hd"}'
```

Restore normal behavior:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"name":"normal"}'
```

Scenario timelines can make deterministic failures evolve over time:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"timeline":[{"at_ms":0,"name":"normal"},{"at_ms":1000,"name":"signal_degraded","mux_id":"src1-11362h-22000-dvbs2"},{"at_ms":2500,"name":"lock_loss","mux_id":"src1-11362h-22000-dvbs2"}]}'
```

Poll `GET /api/scenario` for the active step and `timeline.elapsed_ms`; poll
`GET /api/tuners` when a timeline changes frontend telemetry.

## CLI Companion

Use `satip-labctl` when a shell command is easier than raw `curl`:

```bash
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 wait
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 context
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 status
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 services
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 scenario rtp_loss --service zdf-hd
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 scenario tuner_busy
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 reset
go run ./cmd/satip-labctl smoke --rtsp-host 127.0.0.1 --rtsp-port 554
```

The Docker image includes `/app/satip-labctl`.

## Optional MCP Companion

The Docker image includes `/app/satip-lab-mcp`, and native builds include `bin/satip-lab-mcp`.
It is a separate MCP stdio server that connects to an already-running simulator over HTTP.

```bash
go run ./cmd/satip-lab-mcp --http-url http://127.0.0.1:8875
```

Available tools:

- `satip_agent_context`
- `satip_status`
- `satip_services`
- `satip_scenario`
- `satip_set_scenario`
- `satip_reset`
- `satip_wait_ready`

The main simulator container still starts only `satip-lab`. Run the MCP companion as a separate process when an agent client supports MCP.

## Maintaining This Agent Pack

When implementing new simulator features, update the agent pack in the same PR if the feature changes any of these surfaces:

- Supported profile behavior, including RTSP/RTP, XMLTV, EIT, tuner allocation, catalog loading, or scenarios.
- Configuration variables or CLI flags.
- Lab API endpoints, schemas, or response fields.
- Docker, CI, MCP, `satip-labctl`, smoke probes, or other companion tooling.
- Client-facing test recipes.

At minimum, check and update:

- `/api/agent/context` in `internal/httpserver/agent_context.go`.
- Agent context tests in `internal/httpserver/api_test.go`.
- `docs/agents/README.md` and agent-specific files when workflows change.
- `docs/api.md`, `docs/api-schema.md`, `docs/supported-profile.md`, and `docs/agent-playbook.md` when their contracts change.
