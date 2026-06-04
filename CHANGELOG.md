# Changelog

All notable changes to SAT>IP Lab Server will be documented here.

This project follows semantic versioning for public releases.

## Unreleased

## 1.1.0 - 2026-06-04

- Add `urls.clock` to `/api/agent/context` for self-contained EPG evidence.
- Add runtime `tuner_busy` scenario support through `POST /api/scenario`.
- Add agent-context client expectation hints for deterministic RTP and MPEG-TS fault scenarios.
- Document and advertise the default agent delivery workflow for branch, PR, review, container smoke, release, and merge gates.

## 1.0.0 - 2026-05-23

- Rebrand the project as SAT>IP Lab Server with the `satip-lab` technical identity.
- Publish public multi-arch GHCR images for `linux/amd64` and `linux/arm64`.
- Add funding, security, issue, and release metadata for the public OSS repository.
