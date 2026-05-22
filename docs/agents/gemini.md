# Gemini Instructions

Use `satip-lab` for deterministic SAT>IP client development and CI.

## Bootstrap

```bash
curl -fsS http://127.0.0.1:8875/api/agent/context
```

Use the returned URLs and `test_env` values as the source of truth.

## Scenario Testing

Set a scenario:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"name":"epg_stale"}'
```

Reset:

```bash
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/reset"
curl -fsS -X POST "$SATIP_TEST_HTTP_URL/api/scenario" \
  -H 'Content-Type: application/json' \
  -d '{"name":"normal"}'
```

## Boundaries

- Keep simulator changes focused on the supported lab profile.
- Do not implement full EN 50585 coverage unless explicitly requested.
- Do not place client app code in this repo.
- Run `make test` after Go changes.
