# SAT>IP Lab Server

[![CI](https://github.com/e12media/satip-lab/actions/workflows/ci.yml/badge.svg)](https://github.com/e12media/satip-lab/actions/workflows/ci.yml)
[![Release](https://github.com/e12media/satip-lab/actions/workflows/release.yml/badge.svg)](https://github.com/e12media/satip-lab/actions/workflows/release.yml)
[![Container](https://img.shields.io/badge/ghcr.io-e12media%2Fsatip--lab-blue)](https://github.com/e12media/satip-lab/pkgs/container/satip-lab)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Sponsor](https://img.shields.io/badge/Sponsor-GitHub-ea4aaa)](https://github.com/sponsors/e12media)

**Test SAT>IP clients in CI without SAT>IP hardware.**

SAT>IP Lab Server is a local, deterministic **SAT>IP lab server** for client developers. Run one container, point your app at it, and exercise discovery, M3U import, XMLTV EPG, RTSP sessions, tuner allocation, and RTP MPEG-TS playback without a satellite dish or tuner box in your test loop.

Use it with any SAT>IP client that speaks SSDP, RTSP (`DESCRIBE` / `SETUP` / `PLAY` / `PAUSE` / `TEARDOWN` / `GET_PARAMETER`), and unicast RTP.

Documentation and issues are **English only** (public OSS).

> **Not a production SAT>IP server.** Use real SAT>IP hardware when you need RF-accurate behavior. `satip-lab` is a deterministic **lab server** for development and CI.

## Quick start

Run the public multi-arch image:

```bash
docker run --rm \
  -p 8875:8875 -p 554:554 \
  -e SATIP_LAB_PUBLIC_HOST=127.0.0.1 \
  -e SATIP_LAB_SSDP_PORT=0 \
  ghcr.io/e12media/satip-lab:latest
```

The GHCR image is public and supports `linux/amd64` and `linux/arm64`.

Local development with Compose:

```bash
docker compose up --build
```

Then configure your client:

| Endpoint | URL |
|----------|-----|
| Device description | http://127.0.0.1:8875/desc.xml |
| Channel list (M3U) | http://127.0.0.1:8875/channels.m3u |
| XMLTV EPG | http://127.0.0.1:8875/epg/xmltv.xml |
| Lab clock | http://127.0.0.1:8875/api/clock |
| RTSP base | `rtsp://127.0.0.1:554/` |
| Agent context | http://127.0.0.1:8875/api/agent/context |
| Lab status API | http://127.0.0.1:8875/api/status |
| Lab API reference | [docs/api.md](docs/api.md) |

The bundled M3U lists five DACH-oriented test channels (Das Erste, ZDF, arte, 3sat, phoenix) with SAT>IP tuning query parameters aligned to common Astra 19.2°E presets. Set `SATIP_LAB_CATALOG=fixtures/astra-19.2e-dach.yaml` for a larger 25-service DACH fixture, or mount your own YAML catalog for UI/import scale tests.

The lab model groups those services into muxes and allocates a configurable tuner pool. Services on the same mux can share a tuner; services on different muxes consume additional tuners. When no transport stream file exists, `satip-lab` generates distinct synthetic MPEG-TS payloads per service so client tests can tell channels apart. The Docker image defaults to `SATIP_LAB_SAMPLE_PROFILE=h264_aac_short`, which makes ZDF HD use a short decodable H.264/AAC MPEG-TS test pattern while the other services remain synthetic.

### Docker Desktop (macOS / Windows)

Clients on the host must reach advertised URLs. Set the public host before starting:

```bash
SATIP_LAB_PUBLIC_HOST=host.docker.internal docker compose up --build
```

Use manual server IP `host.docker.internal` (or `127.0.0.1` with published ports) if SSDP multicast is unreliable across the VM boundary.

### Failure scenarios

```bash
SATIP_LAB_SCENARIO=tuner_busy docker compose up
```

`SETUP` returns `503 Service Unavailable` (simulated tuner busy).

Runtime scenarios can be switched while the server is running:

```bash
curl -fsS -X POST http://127.0.0.1:8875/api/scenario \
  -H 'Content-Type: application/json' \
  -d '{"name":"no_signal"}'
```

Supported runtime scenarios are `normal`, `no_signal`, `bad_m3u`, `tuner_busy`, `rtp_stop`, `slow_rtsp`, `malformed_psi`, `rtp_loss`, `rtp_jitter`, `cc_errors`, `epg_gap`, `epg_mismatch`, and `epg_stale`.
Scenarios with a tuned service or mux context can be scoped with optional `service_id` or `mux_id` fields, for example `{"name":"no_signal","service_id":"zdf-hd"}`.

See [docs/api.md](docs/api.md), [docs/epg.md](docs/epg.md), and [docs/supported-profile.md](docs/supported-profile.md).

## Support

SAT>IP Lab Server is free and open source. If it helped you debug a client, avoid hardware setup, or add SAT>IP coverage to CI, you can support maintenance through [GitHub Sponsors](https://github.com/sponsors/e12media).

One-time sponsorships are welcome. Sponsorship is voluntary support and does not include consulting, private support, priority fixes, paid feature commitments, roadmap influence, or SLA. See [docs/sponsorship.md](docs/sponsorship.md).

## For AI agents

- Start with **[AGENTS.md](AGENTS.md)** (canonical context).
- Runtime bootstrap: `GET /api/agent/context`.
- Copy-paste instructions: [docs/agents/](docs/agents/).
- Also: [CLAUDE.md](CLAUDE.md), [docs/agent-playbook.md](docs/agent-playbook.md), [docs/supported-profile.md](docs/supported-profile.md), [docs/api.md](docs/api.md).
- Verify changes: `make test` (required before claiming done).

## Native run (Go)

Requirements: Go 1.25+, optional `ffmpeg`/`ffprobe` for generated test pattern transport streams.

```bash
make test
make run
# or:
./tool/generate_sample_ts.sh   # optional; generates local sample profile assets
go run ./cmd/satip-lab --public-host=127.0.0.1
```

CLI flags mirror environment variables (`--http-port`, `--rtsp-port`, `--scenario`, …).

```bash
make build    # writes bin/satip-lab
```

The build target also writes `bin/satip-labctl`, a small CLI for humans, CI, and coding agents:

```bash
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 wait
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 status
go run ./cmd/satip-labctl --http-url http://127.0.0.1:8875 scenario rtp_loss --service zdf-hd
go run ./cmd/satip-labctl smoke --rtsp-host 127.0.0.1 --rtsp-port 554
```

The build target also writes `bin/satip-lab-mcp`, an optional MCP stdio companion for coding agents. It connects to an already-running simulator:

```bash
go run ./cmd/satip-lab-mcp --http-url http://127.0.0.1:8875
```

## Layout

```text
AGENTS.md               agent context (read first)
docs/                   architecture, profile, CI, playbook
docs/agents/            copy-paste coding agent workflows
cmd/satip-lab/          simulator entrypoint
cmd/satip-lab-mcp/      optional MCP stdio companion
cmd/satip-labctl/       CLI control tool for CI and agents
cmd/satip-lab-smoke/    RTSP/RTP smoke probe
internal/config/        env + flags
fixtures/               optional YAML channel catalogs
internal/channels/      DACH catalog, YAML loader, M3U + device XML
internal/lab/           catalog, tuners, sessions, events
internal/httpserver/    HTTP status, desc.xml, M3U, /api/*
internal/ssdp/          SSDP M-SEARCH + NOTIFY
internal/rtsp/          RTSP + RTP MPEG-TS
internal/smoke/         source-level RTSP/RTP smoke probe
internal/ts/            Looped transport stream
internal/vendorprofile/ RTSP vendor behavior profile definitions
internal/simulator/     coordinates services
Makefile                test, run, docker-up, smoke
```

## Documentation

| Doc | Purpose |
|-----|---------|
| [docs/architecture.md](docs/architecture.md) | Components and flows |
| [docs/agents/](docs/agents/) | Coding agent bootstrap workflows |
| [docs/api.md](docs/api.md) | Lab API endpoints and example payloads |
| [docs/catalog.md](docs/catalog.md) | YAML catalog format and bundled larger fixture |
| [docs/compatibility/servers.md](docs/compatibility/servers.md) | Compatibility corpus and profile evidence levels |
| [docs/epg.md](docs/epg.md) | Deterministic XMLTV EPG contract |
| [docs/vendor-profiles/](docs/vendor-profiles/) | RTSP profile contract and evidence policy |
| [docs/supported-profile.md](docs/supported-profile.md) | What is / isn't simulated |
| [docs/support-matrix.md](docs/support-matrix.md) | Simulator behavior compared with real SAT>IP hardware |
| [docs/ci-integration.md](docs/ci-integration.md) | Client CI service container |
| [docs/roadmap.md](docs/roadmap.md) | SAT>IP lab server roadmap |
| [docs/release-checklist.md](docs/release-checklist.md) | OSS release readiness checks |
| [docs/sponsorship.md](docs/sponsorship.md) | Sponsorship posture and suggested tiers |
| [CONTRIBUTING.md](CONTRIBUTING.md) | PR workflow |

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SATIP_LAB_BIND` | `0.0.0.0` | Bind address |
| `SATIP_LAB_PUBLIC_HOST` | `127.0.0.1` | Host in SSDP `LOCATION` and M3U RTSP URLs |
| `SATIP_LAB_HTTP_PORT` | `8875` | HTTP (XML, M3U, status page) |
| `SATIP_LAB_RTSP_PORT` | `554` | RTSP |
| `SATIP_LAB_PUBLIC_HTTP_PORT` | `0` | Advertised HTTP port; `0` uses `SATIP_LAB_HTTP_PORT` |
| `SATIP_LAB_PUBLIC_RTSP_PORT` | `0` | Advertised RTSP port in M3U; `0` uses `SATIP_LAB_RTSP_PORT` |
| `SATIP_LAB_TUNERS` | `2` | Simulated SAT>IP tuner count |
| `SATIP_LAB_SSDP_PORT` | `1900` | SSDP UDP (`0` disables) |
| `SATIP_LAB_CATALOG` | empty | Optional YAML channel catalog path; empty uses the built-in five-service DACH catalog |
| `SATIP_LAB_TS_PATH` | empty | Optional MPEG-TS file to loop for all services; empty uses distinct generated TS per service |
| `SATIP_LAB_SAMPLE_PROFILE` | `synthetic` | Built-in service media profile when `SATIP_LAB_TS_PATH` is empty: `synthetic`, `h264_aac_short`, or `h264_silent` |
| `SATIP_LAB_PROFILE` | `generic-satip-1.2` | Compatibility profile for SSDP, device XML path/metadata, M3U path, and RTSP behavior |
| `SATIP_LAB_VENDOR_PROFILE` | `spec` | RTSP behavior profile selector alias; `SATIP_LAB_PROFILE` is preferred |
| `SATIP_LAB_EPG_CLOCK` | `fixed:2026-03-29T01:30:00+01:00` | `fixed:<rfc3339>` for deterministic XMLTV output or `real` for demos |
| `SATIP_LAB_SCENARIO` | `normal` | `normal` or `tuner_busy` |

The stable v1 configuration contract is documented in [docs/config-schema.md](docs/config-schema.md) and exposed at runtime from `GET /api/config/schema`.
The stable v1 lab API contract is documented in [docs/api-schema.md](docs/api-schema.md) and exposed at runtime from `GET /api/schema`.

## CI example (GitHub Actions)

```yaml
services:
  satip-lab:
    image: ghcr.io/e12media/satip-lab:latest
    ports:
      - 554:554
      - 8875:8875
    env:
      SATIP_LAB_PUBLIC_HOST: 127.0.0.1
      SATIP_LAB_SSDP_PORT: "0"
      # Optional: SATIP_LAB_CATALOG: /app/fixtures/astra-19.2e-dach.yaml

steps:
  - name: Wait for simulator
    run: |
      for i in $(seq 1 30); do
        curl -fsS http://127.0.0.1:8875/desc.xml && exit 0
        sleep 1
      done
      exit 1
  - name: Run client integration tests
    run: your-client-test-command --satip-host=127.0.0.1
```

If you publish the container on different host ports, also set `SATIP_LAB_PUBLIC_HTTP_PORT` and `SATIP_LAB_PUBLIC_RTSP_PORT` so `desc.xml` and `channels.m3u` advertise URLs your client can actually reach.

To smoke-test RTSP/RTP from a running simulator:

```bash
go run ./cmd/satip-lab-smoke --host 127.0.0.1 --rtsp-port 554
```

When probing a simulator behind Docker Desktop or another NAT boundary, pass the host IP reachable from the container:

```bash
go run ./cmd/satip-lab-smoke --host 127.0.0.1 --rtsp-port 554 --rtp-destination <host-ip-from-container>
```

Tagged releases publish `ghcr.io/e12media/satip-lab:<version>`, `<major>.<minor>`, and `latest`. The public GHCR image supports `linux/amd64` and `linux/arm64`.

## Alternatives

| Tool | Role |
|------|------|
| [minisatip](https://github.com/catalinii/minisatip) | Full SAT>IP server (needs tuner for real TV) |
| [TVHeadend](https://tvheadend.org/) | Full TV stack; `--tsfile` for file-based lab |
| [sat-ip-proxy](https://github.com/alexte/sat-ip-proxy) | RTSP proxy / debugger |
| VLC (UPnP) | Manual discovery check |

`satip-lab` targets **one command, predictable channels, CI-friendly** behavior.

## License

MIT — see [LICENSE](LICENSE).
