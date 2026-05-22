# GitHub Copilot instructions

This repository is **satip-lab**: a Go SAT>IP **lab server** for testing SAT>IP clients.

- Read [AGENTS.md](../AGENTS.md) for full context.
- Not a production TV server; not full SAT>IP spec.
- Entry point: `cmd/satip-lab/main.go`; lab model in `internal/lab/`; JSON API in `docs/api.md`.
- Run `make test` after changes.
- RTSP uses `\r\n\r\n`; do not use `println` for RTSP responses.
- See `docs/supported-profile.md` for guaranteed behavior; `docs/roadmap.md` for planned lab scenarios.
- All text in English (US spelling).

Working rules:

1. Don’t assume. Don’t hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what you must. Clean up only your own mess.
4. Define success criteria. Loop until verified.
