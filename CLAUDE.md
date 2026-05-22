# satip-lab

This file exists for tools that read `CLAUDE.md` at the repo root.

**Full agent instructions:** [AGENTS.md](AGENTS.md)

Quick facts:

- Go SAT>IP **lab server** (not production server)
- `make test` before claiming done
- Entry: `cmd/satip-lab/main.go`; lab API: `docs/api.md`
- Do not implement full SAT>IP spec beyond the supported profile
- **English only** for all docs, comments, and PR text (US spelling)

Working rules:

1. Don’t assume. Don’t hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what you must. Clean up only your own mess.
4. Define success criteria. Loop until verified.
