# Cursor Instructions

Use these notes when working in a client repo or this simulator repo with Cursor.

## Runtime Contract

Fetch:

```bash
curl -fsS http://127.0.0.1:8875/api/agent/context
```

Then use:

```bash
SATIP_TEST_HTTP_URL=http://127.0.0.1:8875
SATIP_TEST_RTSP_URL=rtsp://127.0.0.1:554/
```

Client integration tests should use those variables rather than assuming ports.

## Suggested Client Assertions

- Device XML is reachable.
- M3U contains SAT>IP RTSP URLs and expected DACH channels.
- XMLTV has deterministic programmes and channel ids matching M3U.
- RTSP `SETUP` and `PLAY` succeed in `normal`.
- Client error handling works for at least one relevant runtime scenario.

## Repository Edits

If editing `satip-lab`, read `AGENTS.md` first and keep changes inside the simulator scope. Run `make test` after Go changes.
