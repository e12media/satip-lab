# satip-lab Agent Context

Read this file first. Deeper references live under `docs/`.

## Language

This is a public OSS project: **all documentation, comments, commit messages, PR descriptions, and agent responses must be in English** (US spelling preferred: e.g. behavior, not behaviour). German channel names in the DACH preset (e.g. "Das Erste HD") are proper nouns and stay as-is.

## What This Repo Is

OSS **SAT>IP lab server** for developers building and testing SAT>IP clients.

- Simulates enough of a SAT>IP server for **discovery, M3U, XMLTV EPG, RTSP session, tuner allocation, RTP MPEG-TS**.
- **Not** a production TV server and **not** full EN 50585 coverage.

## Stack

| Layer | Location |
|-------|----------|
| Entry | `cmd/satip-lab/main.go` |
| Smoke probe | `cmd/satip-lab-smoke/main.go`, `internal/smoke/` |
| Config (env + flags) | `internal/config/` |
| DACH channels + YAML catalog + M3U/XML | `internal/channels/`, `fixtures/` |
| XMLTV EPG | `internal/epg/` |
| Lab catalog + tuners + sessions | `internal/lab/` |
| HTTP (desc.xml, M3U, status) | `internal/httpserver/` |
| SSDP | `internal/ssdp/` (uses `golang.org/x/net/ipv4`) |
| RTSP + RTP | `internal/rtsp/` |
| MPEG-TS loop | `internal/ts/` |
| Vendor profiles | `internal/vendorprofile/` |
| Wiring | `internal/simulator/` |

- **Go 1.25+** (`go.mod`)
- Non-stdlib deps: `golang.org/x/net` (SSDP multicast), `gopkg.in/yaml.v3` (catalog files)
- Docker: multi-stage, `ffmpeg` generates `assets/*.ts` media assets at build time

## Supported Simulation Profile

See `docs/supported-profile.md`. Summary:

| Feature | Simulated |
|---------|-----------|
| SSDP M-SEARCH → `urn:ses-com:device:SatIPServer:1` | Yes |
| Device description XML | Yes |
| M3U with SAT>IP RTSP URLs | Yes (5 default DACH channels or YAML catalog) |
| Deterministic XMLTV EPG | Yes (`/epg/xmltv.xml`, `/api/clock`) |
| RTSP OPTIONS / SETUP / PLAY / TEARDOWN | Yes |
| RTP/UDP MPEG-TS (PT 33) after PLAY | Yes, including synthetic PAT/PMT and minimal EIT p/f |
| H.264/AAC decodable sample profile | Yes (ZDF HD only; Docker default) |
| Tuner allocation / same-mux sharing | Yes |
| Lab JSON API (`/api/*`, runtime scenarios) | Yes — see `docs/api.md` |
| Vendor profile framework | Yes (`spec` only; non-spec profiles require evidence) |
| TCP interleaved RTSP | No (v0) |
| Real RF transponder tuning | No (catalog-backed synthetic lab model) |
| Vendor-specific quirks | No |

## Environment Variables

| Variable | Default | Notes |
|----------|---------|-------|
| `SATIP_LAB_BIND` | `0.0.0.0` | Listen address |
| `SATIP_LAB_PUBLIC_HOST` | `127.0.0.1` | In SSDP `LOCATION` and M3U RTSP URLs |
| `SATIP_LAB_HTTP_PORT` | `8875` | |
| `SATIP_LAB_RTSP_PORT` | `554` | |
| `SATIP_LAB_PUBLIC_HTTP_PORT` | `0` | Advertised HTTP port; `0` uses `SATIP_LAB_HTTP_PORT` |
| `SATIP_LAB_PUBLIC_RTSP_PORT` | `0` | Advertised RTSP port; `0` uses `SATIP_LAB_RTSP_PORT` |
| `SATIP_LAB_TUNERS` | `2` | Simulated tuner count |
| `SATIP_LAB_SSDP_PORT` | `1900` | `0` disables SSDP |
| `SATIP_LAB_CATALOG` | empty | Optional YAML channel catalog path; empty uses the five-service built-in catalog |
| `SATIP_LAB_TS_PATH` | empty | Optional file loop; empty generates distinct TS per service |
| `SATIP_LAB_SAMPLE_PROFILE` | `synthetic` | `synthetic`, `h264_aac_short`, or `h264_silent`; Docker image defaults to `h264_aac_short` |
| `SATIP_LAB_PROFILE` | `generic-satip-1.2` | Compatibility profile for SSDP, device XML path/metadata, M3U path, and RTSP behavior |
| `SATIP_LAB_VENDOR_PROFILE` | `spec` | RTSP behavior profile selector alias; `SATIP_LAB_PROFILE` is preferred |
| `SATIP_LAB_EPG_CLOCK` | `fixed:2026-03-29T01:30:00+01:00` | `fixed:<rfc3339>` or `real`; default crosses Europe/Berlin DST |
| `SATIP_LAB_SCENARIO` | `normal` | `tuner_busy` → all SETUP return 503; lab can also return 503 when tuners exhausted |

## Commands (verification before claiming done)

```bash
make test          # go test ./...
make lint          # go vet ./...
make run           # local server
make docker-up     # compose build + up
make smoke         # curl desc.xml + m3u and verify RTSP/RTP (server must be running)
```

**Do not claim success without running `make test` after Go changes.**

## Working Rules

1. Don’t assume. Don’t hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what you must. Clean up only your own mess.
4. Define success criteria. Loop until verified.
5. **Minimize scope** — fix the simulator, not full SAT>IP spec or client application code.
6. **Preserve wire compatibility** — RTSP responses use `\r\n\r\n`; CSeq and Session headers must stay correct.
7. **DACH channel table** — changes to `internal/channels/catalog.go` may break client tests that depend on stable M3U entries; document in PR.
8. **No secrets** in repo; no force-push to `main`.
9. **Branches** — use `codex/` prefix for agent branches.
10. **RTP loop** — `internal/ts` must not block the event loop (use ticker/delay between chunks).
11. **Docker** — if `go.mod` min version changes, update `Dockerfile` golang image and `.github/workflows/ci.yml`.
12. **Agent pack** — if a change adds or modifies client-facing lab behavior, config, scenarios, catalog/EPG/EIT behavior, API contracts, smoke probes, Docker/CI workflows, MCP, or `satip-labctl`, update `/api/agent/context` and `docs/agents/` in the same PR.

## Agent Delivery Workflow

Default implementation workflow for agent-authored changes:

1. Create or switch to a `codex/` branch before editing.
2. Implement the smallest scoped change with tests.
3. Run `make test` and `make lint`.
4. For runtime behavior, Docker, CI, media generation, or advertised lab contract changes, build and smoke-test the container:
   ```bash
   make docker-up
   make smoke
   make docker-down
   ```
5. Open a PR with the verification evidence and any client-facing compatibility notes.
6. Request or spawn a PR review pass before merge; address actionable findings.
7. Re-run the relevant tests after review fixes. If the container path was required before review, rebuild and smoke-test it again.
8. Publish release containers and merge only when explicitly requested by a maintainer or through the repository release workflow.

For documentation-only changes, keep the branch/PR/review discipline, but container build/smoke is optional unless the docs change Docker, CI, release, or client runtime instructions.

## Out of Scope (do not add without explicit request)

- Full SAT>IP spec, real broadcast EPG ingestion, full DVB EIT/SI generation
- TCP interleaved RTSP, HTTPS, authentication
- Replacing minisatip/TVHeadend as a real server
- Client application code (belongs in client repos)

## Key Docs

- Architecture: `docs/architecture.md`
- Supported profile: `docs/supported-profile.md`
- Lab API: `docs/api.md`
- Catalog format: `docs/catalog.md`
- EPG contract: `docs/epg.md`
- Vendor profiles: `docs/vendor-profiles/`
- Coding agents: `docs/agents/README.md`
- Agent playbook: `docs/agent-playbook.md`
- Roadmap: `docs/roadmap.md`
- CI for clients: `docs/ci-integration.md`

## Repository

- Org: `e12media`
- Repo: `https://github.com/e12media/satip-lab`
- Default branch: `main`
- License: MIT
