# Agent Playbook

How AI coding agents should work in this repository.

## Start of session

1. Read [AGENTS.md](../AGENTS.md).
2. Create or switch to a `codex/` branch before editing.
3. Skim [supported-profile.md](supported-profile.md) if touching protocol behavior.
4. Skim [api.md](api.md) if touching lab state, tuners, sessions, or HTTP `/api/*`.
5. Run `make test` to confirm baseline (after checkout) when the work is more than a small docs-only edit.

## Delivery workflow

Use this flow for agent-authored implementation work:

1. Branch: work on a `codex/` branch, never directly on `main`.
2. Implement: keep the change scoped and include tests for behavior changes.
3. Verify locally: run `make test` and `make lint`.
4. Verify container when runtime behavior, Docker, CI, media generation, or advertised lab contracts changed:
   ```bash
   make docker-up
   make smoke
   make docker-down
   ```
5. Open a PR with test output, container smoke evidence when applicable, and client-facing compatibility notes.
6. Request or spawn a PR review pass. Implement actionable review issues only after verifying they are correct.
7. Re-run `make test`, `make lint`, and the container smoke path again when that path was required.
8. Publish containers and merge only with explicit maintainer approval or the repository release workflow.

Documentation-only changes still use a branch and PR. Container smoke is optional unless the documentation changes Docker, CI, release, or client runtime instructions.

## Common tasks

| Task | Where to edit | Verify |
|------|---------------|--------|
| Add default channel | `internal/channels/catalog.go` (mux layout follows lab catalog) | `make test`, `curl …/channels.m3u`, `curl …/api/services` |
| Add catalog fixture | `fixtures/`, `docs/catalog.md` | `SATIP_LAB_CATALOG=... make run`, `curl …/api/catalog` |
| Agent-facing capability | `internal/httpserver/agent_context.go`, `docs/agents/` | `internal/httpserver/api_test.go`, `curl …/api/agent/context` |
| Lab catalog / mux rules | `internal/lab/catalog.go` | `internal/lab/lab_test.go`, `curl …/api/catalog` |
| Tuner allocation | `internal/lab/manager.go`, `internal/rtsp/server.go` | `internal/lab/lab_test.go`, `internal/simulator/integration_test.go` |
| Lab HTTP API | `internal/httpserver/server.go` | `internal/httpserver/api_test.go`, [api.md](api.md) |
| New env flag | `internal/config/config.go`, `cmd/satip-lab/main.go` | `config_test.go`, README, AGENTS.md |
| Startup RTSP scenario | `internal/config`, `internal/rtsp/server.go` | `integration_test.go` (`SATIP_LAB_SCENARIO=tuner_busy`) |
| Compatibility profile | `internal/vendorprofile/`, `internal/httpserver/`, `internal/ssdp/`, `internal/rtsp/server.go`, `docs/compatibility/`, `docs/vendor-profiles/` | `internal/vendorprofile`, `internal/channels`, `internal/httpserver`, `internal/ssdp`, `internal/rtsp` tests, trace-backed docs |
| Runtime lab scenario | `internal/lab/manager.go`, `internal/httpserver/server.go`, `internal/rtsp/server.go` | `internal/lab/lab_test.go`, `internal/httpserver/api_test.go` |
| SSDP change | `internal/ssdp/server.go` | manual M-SEARCH or integration |
| RTP / per-service TS | `internal/ts/source.go`, `internal/rtsp/server.go` | `make smoke`, integration RTP tests |
| Docker/CI | `Dockerfile`, `.github/workflows/ci.yml` | `docker compose build` |

## Checklist before marking work complete

- [ ] Working on a `codex/` branch, not `main`
- [ ] `make test` passes
- [ ] `make lint` passes (or `go vet ./...`)
- [ ] Container build/smoke completed when runtime behavior, Docker, CI, media generation, or advertised lab contracts changed
- [ ] PR opened or explicitly deferred by the maintainer
- [ ] PR review pass requested or spawned before merge
- [ ] If profile or lab API changed: updated `docs/supported-profile.md` and/or `docs/api.md`
- [ ] If new env var: updated README + AGENTS.md table
- [ ] If client-facing behavior, config, scenarios, catalog/EPG/EIT, or tooling changed: updated `/api/agent/context` and `docs/agents/`
- [ ] No unrelated refactors
- [ ] Did not add client application code to this repo

## RTSP pitfalls (learned from implementation)

1. **Line endings** — use `\r\n` join for RTSP bodies, not `println`.
2. **PLAY must not block** — TS loop needs delays; infinite sync loop blocks PLAY response.
3. **Session header** — parse `Session: id;timeout=60` before lookup.
4. **Tests** — read full RTSP response until `\r\n\r\n`; one socket per exchange in tests is fine.

## What to avoid

- Expanding into “full SAT>IP server”
- Adding heavy dependencies without justification
- Breaking default ports (8875, 554, 1900) without major version note
- Committing generated `assets/*.ts` media assets (gitignored)

## Working with client repos

Client application changes belong in the client repository. Change satip-lab only when the **simulator** needs new behavior for tests.

Cross-repo test flow:

1. `docker compose up` in satip-lab
2. Point the client at `127.0.0.1` (manual IP) or use a CI service container — see [ci-integration.md](ci-integration.md)
