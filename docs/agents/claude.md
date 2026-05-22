# Claude Instructions

Use `satip-lab` as a deterministic SAT>IP lab server, not as a full hardware replacement.

## First Steps

1. Fetch `GET /api/agent/context`.
2. Read the returned documentation paths.
3. Configure the client under test with `SATIP_TEST_HTTP_URL` and `SATIP_TEST_RTSP_URL`.
4. Reset simulator state before isolated tests.

## Useful Commands

```bash
curl -fsS http://127.0.0.1:8875/api/agent/context
curl -fsS -X POST http://127.0.0.1:8875/api/reset
curl -fsS -X POST http://127.0.0.1:8875/api/scenario \
  -H 'Content-Type: application/json' \
  -d '{"name":"bad_m3u"}'
```

## Guardrails

- Do not expand this repository into a full SAT>IP server.
- Do not add client application code here.
- Treat `internal/channels/catalog.go` as a stable client-test contract.
- Preserve RTSP wire compatibility, including CRLF response framing.
- Run `make test` after Go changes.

## Optional MCP

Start the companion with:

```bash
go run ./cmd/satip-lab-mcp --http-url http://127.0.0.1:8875
```

Use its tools for status, reset, readiness, service listing, and scenario changes.
