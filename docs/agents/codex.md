# Codex Instructions

Use `satip-lab` as a deterministic SAT>IP lab server for client integration tests.

## Bootstrap

```bash
curl -fsS http://127.0.0.1:8875/api/agent/context
```

Read `AGENTS.md`, `docs/supported-profile.md`, and `docs/api.md` before changing simulator behavior.
If the work changes supported lab behavior, config, scenarios, catalog loading, EPG/EIT, companion tools, or client test workflows, update `docs/agents/` and `/api/agent/context` in the same PR.

## Development Workflow

Use the repository delivery flow unless the maintainer explicitly scopes the task differently:

1. Create or switch to a `codex/` branch before editing.
2. Implement the smallest scoped change, with tests for behavior changes.
3. Run `make test` and `make lint`.
4. If runtime behavior, Docker, CI, media generation, or advertised lab contracts changed, run:
   ```bash
   make docker-up
   make smoke
   make docker-down
   ```
5. Open a PR with verification evidence.
6. Spawn or request a PR review pass, implement confirmed review issues, then rerun the relevant tests.
7. Publish containers and merge only when explicitly requested by a maintainer or through the release workflow.

## Client Test Environment

Use values from `/api/agent/context` when available:

```bash
export SATIP_TEST_HTTP_URL=http://127.0.0.1:8875
export SATIP_TEST_RTSP_URL=rtsp://127.0.0.1:554/
```

Prefer these variables over hard-coded URLs in client tests.

For larger catalog tests, use `SATIP_LAB_CATALOG=fixtures/astra-19.2e-dach.yaml` locally or `/app/fixtures/astra-19.2e-dach.yaml` in Docker. For compatibility hardening, run the same client tests with `SATIP_LAB_PROFILE=tvheadend`, `SATIP_LAB_PROFILE=minisatip`, or another documented profile. For guide fallback tests, remember that generated synthetic TS includes DVB EIT present/following on PID `0x0012`.

## Agent Rules

- Reset state between independent tests with `POST /api/reset`.
- Use runtime scenarios for deterministic error cases, including frontend telemetry scenarios for signal-quality UI and retry tests.
- Use `/api/status.hardware` for lab-owned hardware-style uptime, identity, stream, tuner, and counter assertions; do not treat it as a vendor management API.
- Use scenario timelines when a client test needs failure behavior to evolve over elapsed milliseconds.
- Keep client application changes in the client repository.
- If you edit Go code in this repo, run `make test` before reporting success.
- If you change API behavior, update `docs/api.md` and `docs/api-schema.md`.

## MCP

Prefer `satip-labctl` for shell-native workflows:

```bash
go run ./cmd/satip-labctl --http-url "$SATIP_TEST_HTTP_URL" wait
go run ./cmd/satip-labctl --http-url "$SATIP_TEST_HTTP_URL" status
```

When MCP is useful, run:

```bash
go run ./cmd/satip-lab-mcp --http-url "$SATIP_TEST_HTTP_URL"
```

Use `satip_agent_context` first, then `satip_wait_ready`, `satip_status`, and scenario tools as needed.
