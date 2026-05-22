# satip-lab Architecture

## Purpose

Provide a **deterministic SAT>IP lab server surface** for client development and CI without satellite hardware.

## Component diagram

```text
                    ┌─────────────────┐
                    │  cmd/satip-lab  │
                    │  flags + env    │
                    │ catalog/profile │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │ internal/       │
                    │ simulator       │
                    └────────┬────────┘
         ┌───────────────────┼───────────────────┐
         │                   │                   │
  ┌──────▼──────┐    ┌───────▼───────┐   ┌──────▼──────┐
  │ httpserver  │    │ rtsp.Server   │   │ ssdp.Server │
  │ :8875       │    │ :554          │   │ :1900/udp   │
  └──────┬──────┘    └───────┬───────┘   └─────────────┘
         │                   │
  desc.xml, M3U, API   SETUP/PLAY ──► RTP/UDP ──► client
         │                   │
         │            ┌──────▼──────┐
         │            │ lab.Manager │
         │            │ mux/tuners  │
         └────────────┤ sessions    │
                      └──────┬──────┘
                             │
                      ┌──────▼──────┐
                      │ ts.Source   │
                      │ file, sample│
                      │ or synth TS │
                      └─────────────┘
```

## Request flows

### Discovery (SSDP)

1. Client sends M-SEARCH for `urn:ses-com:device:SatIPServer:1`.
2. `ssdp.Server` replies with `LOCATION: http://{PUBLIC_HOST}:{HTTP_PORT}/desc.xml`.
3. Client fetches device description from `httpserver`.

### Channel list

1. `simulator.New` loads the built-in five-service DACH catalog or the YAML file from `SATIP_LAB_CATALOG`.
2. Client GET `/channels.m3u`.
3. `httpserver` renders the active lab catalog as SAT>IP query parameters on RTSP URLs.

### Playback

1. Client RTSP `SETUP` with `Transport: RTP/AVP;unicast;client_port=…`.
2. `lab.Manager` validates the tuning query, applies any active runtime scenario, allocates or shares a tuner for the requested mux, and records the session.
3. Server returns `Session` id.
4. Client `PLAY` → `rtsp.Server` starts goroutine reading `ts.Source`, sending RTP MP2T (PT 33) to client UDP port.
5. Client `PLAY` requests with `pids`, `addpids`, or `delpids` update the lab session PID set.
6. Client `PAUSE` stops the RTP goroutine while keeping lab session/tuner state.
7. Client `GET_PARAMETER` validates the RTSP session as a keepalive.
8. A background RTSP session reaper expires sessions after the advertised timeout and releases RTP/tuner state.
9. Client `TEARDOWN` stops the RTP goroutine and releases lab session/tuner state.

### Lab API

- `GET /api/status` returns tuners, sessions, and recent events.
- `GET /api/catalog`, `/api/muxes`, and `/api/services` expose the active lab catalog.
- `GET /api/tuners`, `/api/sessions`, and `/api/events` return individual state slices.
- `GET /api/scenario` and `POST /api/scenario` inspect or switch runtime lab scenarios such as `no_signal`.
- `POST /api/reset` clears lab sessions, tuner state, and records a reset event.

## Concurrency

- One TCP connection per RTSP client handler; requests parsed from buffer.
- Lab sessions, tuner state, and events live in `lab.Manager` guarded by `sync.Mutex`.
- RTSP UDP streaming sessions are stored in `rtsp.Server` and guarded by a separate mutex.
- Each PLAY session owns a UDP socket and a stop channel.

## Docker image

1. **ts-asset** stage: `ffmpeg` builds generated MPEG-TS assets under `assets/`.
2. **build** stage: `go build` static binary.
3. **runtime** stage: binary + generated MPEG-TS assets + bundled catalog fixtures, no Go toolchain. Docker defaults to `SATIP_LAB_SAMPLE_PROFILE=h264_aac_short` so ZDF HD is decodable out of the box.

## Extension points

| Change | Touch |
|--------|--------|
| New built-in channel | `internal/channels/catalog.go` |
| New optional catalog fixture | `fixtures/`, `docs/catalog.md` |
| Lab mux/tuner/session behavior | `internal/lab/` |
| New env failure scenario | `internal/config` + `internal/rtsp/server.go` |
| New runtime lab scenario | `internal/lab/manager.go` + `internal/rtsp/server.go` + `internal/httpserver/server.go` |
| New compatibility profile | `internal/vendorprofile/`, `internal/httpserver/`, `internal/ssdp/`, `internal/rtsp/`, `docs/compatibility/`, `docs/vendor-profiles/` |
| New HTTP endpoint | `internal/httpserver/server.go` |
| Transport mode (e.g. TCP) | `internal/rtsp/` (new subsystem) |
| RTSP/RTP smoke coverage | `internal/smoke/`, `cmd/satip-lab-smoke/` |
