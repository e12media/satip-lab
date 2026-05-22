# Channel Catalogs

`satip-lab` starts with a deterministic five-service DACH catalog by default. For larger import, guide, tuner, and UI tests, set `SATIP_LAB_CATALOG` or `--catalog` to a YAML file.

```bash
SATIP_LAB_CATALOG=fixtures/astra-19.2e-dach.yaml go run ./cmd/satip-lab
```

In Docker, the bundled larger fixture is available at `/app/fixtures/astra-19.2e-dach.yaml`:

```bash
docker run --rm \
  -p 8875:8875 -p 554:554 \
  -e SATIP_LAB_PUBLIC_HOST=127.0.0.1 \
  -e SATIP_LAB_SSDP_PORT=0 \
  -e SATIP_LAB_CATALOG=/app/fixtures/astra-19.2e-dach.yaml \
  ghcr.io/e12media/satip-lab:latest
```

## Format

```yaml
services:
  - id: custom-news-hd
    number: 201
    name: Custom News HD
    group: Lab
    tvg_id: custom-news.example
    src: 1
    freq: 12188
    pol: h
    sr: 27500
    msys: dvbs2
    pids: [0, 17, 8100, 8101, 8102]
```

| Field | Meaning |
|-------|---------|
| `id` | Stable simulator service id used by `/api/catalog`, scenarios, events, and generated TS markers. |
| `number` | Channel number used in `#EXTINF` and guide ordering. Must be unique. |
| `name` | Display name used as M3U `tvg-name` and XMLTV `display-name`. |
| `group` | M3U `group-title`. |
| `tvg_id` | M3U and XMLTV channel id. Must be unique. |
| `src`, `freq`, `pol`, `sr`, `msys` | SAT>IP-style tuning tuple used for RTSP service matching and mux grouping. |
| `pids` | At least five PIDs: PAT, SDT, PMT, video, audio. Values must be `0..8191`. |

`pol` must be `h` or `v`, and `msys` must be `dvbs` or `dvbs2`. The first two PID entries must be `0` and `17` because the current lab model fixes PAT and SDT at those PIDs while mapping the PMT/video/audio PIDs from the catalog.

Services with the same `src`/`freq`/`pol`/`sr`/`msys` tuple share one simulated mux and can share one tuner. The simulator validates the catalog before opening HTTP or RTSP sockets; invalid files fail startup with field-specific errors. Unknown YAML fields are rejected so typos do not silently change test fixtures.

## Bundled Fixture

`fixtures/astra-19.2e-dach.yaml` contains 25 DACH-oriented services across public, regional, Austrian, Swiss, and private groups. It is deterministic lab data shaped like common ASTRA 19.2E lineups, not a live broadcast dump.

The first five entries keep the default service ids and `tvg_id` values:

| Service id | TVG id |
|------------|--------|
| `das-erste-hd` | `daserste.de` |
| `zdf-hd` | `zdf.de` |
| `arte-hd` | `arte.de` |
| `3sat-hd` | `3sat.de` |
| `phoenix-hd` | `phoenix.de` |
