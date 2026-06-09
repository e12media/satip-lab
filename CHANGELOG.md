# Changelog

All notable changes to SAT>IP Lab Server will be documented here.

This project follows semantic versioning for public releases.

## Unreleased

- Add hardware-style status, frontend lifecycle telemetry, and deterministic hardware fault scenarios.
- Add capture-backed compatibility evidence tooling and DVB SI basics with SDT/NIT generation.
- Add deterministic multi-server topology fixtures through `SATIP_LAB_TOPOLOGY` and `/api/topology`.
- Add RTP playback observability fields on `/api/sessions` and bounded RTP lifecycle events.
- Add `/api/playback/diagnostics` for per-session playback diagnostics, packet rate, and intentional impairment flags.

## 1.1.0 - 2026-06-04

- Add `urls.clock` to `/api/agent/context` for self-contained EPG evidence.
- Add runtime `tuner_busy` scenario support through `POST /api/scenario`.
- Add agent-context client expectation hints for deterministic RTP and MPEG-TS fault scenarios.
- Document and advertise the default agent delivery workflow for branch, PR, review, container smoke, release, and merge gates.

## 1.0.0 - 2026-05-23

- Rebrand the project as SAT>IP Lab Server with the `satip-lab` technical identity.
- Publish public multi-arch GHCR images for `linux/amd64` and `linux/arm64`.
- Add funding, security, issue, and release metadata for the public OSS repository.
