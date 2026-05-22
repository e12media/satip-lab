# Contributing to satip-lab

## Language

All contributions must be in **English**: docs, code comments, issues, PRs, and commit messages. Use US spelling in project text where it differs from UK spelling (e.g. behavior).

## For humans and AI agents

1. Read [AGENTS.md](AGENTS.md) and [docs/agent-playbook.md](docs/agent-playbook.md).
2. Create a branch: `codex/<short-topic>`.
3. Run `make test` (and `make smoke` if you changed HTTP/RTSP behavior).
4. Open a PR with what changed in the **simulation profile** if behavior is user-visible.

## Development setup

```bash
go test ./...
./tool/generate_sample_ts.sh   # optional; Docker build generates TS
go run ./cmd/satip-lab
```

## Pull requests

- One logical change per PR.
- Update `docs/supported-profile.md` when adding/removing simulated behavior.
- Keep Docker image building (`docker compose up --build`).

## Code style

- Standard Go formatting (`gofmt`).
- Prefer stdlib; justify new dependencies in the PR.
- Integration tests live next to packages (`*_test.go`) or under `internal/simulator/`.
