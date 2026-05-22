# Lab API

The lab API is an in-memory test and debugging surface. It is intentionally small and JSON-only.

## `GET /api/config/schema`

Returns the versioned configuration schema. See [config-schema.md](config-schema.md).

## `GET /api/agent/context`

Returns a coding-agent bootstrap document with advertised URLs, test environment variables, catalog summary, current runtime state, supported scenarios, recommended checks, and documentation paths:

```json
{
  "version": "1.0",
  "urls": {
    "http_base_url": "http://127.0.0.1:8875",
    "rtsp_base_url": "rtsp://127.0.0.1:554/",
    "device_xml": "http://127.0.0.1:8875/desc.xml",
    "m3u": "http://127.0.0.1:8875/channels.m3u",
    "xmltv": "http://127.0.0.1:8875/epg/xmltv.xml",
    "schema": "http://127.0.0.1:8875/api/schema",
    "config_schema": "http://127.0.0.1:8875/api/config/schema",
    "status": "http://127.0.0.1:8875/api/status"
  },
  "test_env": {
    "SATIP_TEST_HTTP_URL": "http://127.0.0.1:8875",
    "SATIP_TEST_RTSP_URL": "rtsp://127.0.0.1:554/"
  },
  "catalog": {
    "service_count": 5,
    "mux_count": 4,
    "source": "built_in",
    "fixture_path": "fixtures/astra-19.2e-dach.yaml"
  },
  "features": {
    "custom_catalogs": true,
    "compatibility_profiles": true,
    "xmltv_epg": true,
    "eit_present_following": true,
    "rtsp_rtp_smoke": true,
    "runtime_scenarios": true
  },
  "runtime": {
    "tuners": 2,
    "scenario": "normal",
    "profile": "generic-satip-1.2",
    "readiness_path": "/api/agent/context",
    "reset_path": "/api/reset",
    "scenario_path": "/api/scenario"
  },
  "compatibility": {
    "active_profile": "generic-satip-1.2",
    "available_profiles": [
      "generic-satip-1.2",
      "spec",
      "minisatip",
      "tvheadend",
      "triax-tss400",
      "telestar-digibit-r1",
      "kathrein-exip",
      "digital-devices-octopus-net"
    ],
    "corpus_path": "docs/compatibility/servers.md"
  }
}
```

Use this endpoint as the readiness probe for agent-driven client tests.

## `GET /api/schema`

Returns the versioned lab API schema. See [api-schema.md](api-schema.md).

## `GET /api/clock`

Returns the current lab clock used for deterministic XMLTV generation:

```json
{
  "mode": "fixed",
  "now": "2026-03-29T01:30:00+01:00",
  "tz": "Europe/Berlin"
}
```

See [epg.md](epg.md) for clock semantics.

## `GET /epg/xmltv.xml`

Returns deterministic XMLTV for the active catalog. The endpoint sets `Content-Type: application/xml; charset=utf-8` and `Last-Modified` relative to the lab clock.

See [epg.md](epg.md) for the XMLTV contract, timestamp format, schedule density, and EPG scenarios.

## `GET /api/status`

Returns the full lab state:

```json
{
  "tuners": [
    {
      "id": 1,
      "state": "idle"
    }
  ],
  "sessions": [],
  "events": []
}
```

## `GET /api/catalog`

Returns the active catalog with `muxes` and `services`.

## `GET /api/muxes`

Returns mux/transponder-like lab entries:

```json
[
  {
    "id": "src1-11494h-22000-dvbs2",
    "src": 1,
    "freq": 11494,
    "pol": "h",
    "sr": 22000,
    "msys": "dvbs2"
  }
]
```

## `GET /api/services`

Returns services/channels:

```json
[
  {
    "id": "das-erste-hd",
    "number": 1,
    "name": "Das Erste HD",
    "group": "DE",
    "tvg_id": "daserste.de",
    "mux_id": "src1-11494h-22000-dvbs2",
    "service_id": 1001,
    "pmt_pid": 5100,
    "video_pid": 5101,
    "audio_pid": 5102
  }
]
```

## `GET /api/tuners`

Returns tuner state. A tuned tuner includes its mux and session ids.

## `GET /api/sessions`

Returns active RTSP lab sessions.

## `GET /api/events`

Returns recent lab events such as `session_setup`, `play_started`, `session_closed`, `tuner_busy`, and `reset`.

## `GET /api/scenario`

Returns the active runtime scenario:

```json
{
  "name": "normal",
  "description": "Normal SAT>IP simulator behavior."
}
```

## `POST /api/scenario`

Changes the active runtime scenario:

```json
{
  "name": "epg_gap",
  "service_id": "arte-hd",
  "duration_min": 60
}
```

Supported values:

| Name | Behavior |
| --- | --- |
| `normal` | Normal SAT>IP simulator behavior. |
| `no_signal` | Valid RTSP `SETUP` requests return `503 Service Unavailable` with `Reason: no signal`; no tuner or session is allocated. |
| `bad_m3u` | `/channels.m3u` returns deliberately malformed playlist content with a stable `satip-lab:bad_m3u` marker and no usable RTSP URLs. |
| `rtp_stop` | RTSP `SETUP` and `PLAY` succeed, then RTP stops after a short deterministic packet burst without an explicit `TEARDOWN`. |
| `slow_rtsp` | RTSP responses are delayed by a deterministic 250 ms. |
| `malformed_psi` | RTP keeps valid packet framing while generated PAT/PMT table headers are deliberately corrupted. |
| `rtp_loss` | RTP drops every third packet after `PLAY`. |
| `rtp_jitter` | RTP adds deterministic 40 ms timing jitter to every third packet. |
| `cc_errors` | RTP keeps packet framing while MPEG-TS continuity counters are deliberately corrupted. |
| `epg_gap` | `/epg/xmltv.xml` omits a deterministic programme window for a targeted service or mux. |
| `epg_mismatch` | `/epg/xmltv.xml` changes the ZDF HD XMLTV channel id to `zdf-mismatch.invalid`. |
| `epg_stale` | `/epg/xmltv.xml` returns normal XMLTV content with `Last-Modified` set 48 hours before the lab clock. |

Unknown scenario names return `400 Bad Request` and leave the active scenario unchanged.
Optional `service_id` and `mux_id` fields scope tune-aware RTSP/RTP scenarios, and `epg_gap`, to one service, one mux, or the intersection of both. Untargeted scenarios remain global. Unknown service or mux IDs return `400 Bad Request` and leave the active scenario unchanged.

HTTP-only `bad_m3u`, XMLTV-wide `epg_mismatch` and `epg_stale`, pre-tune `slow_rtsp`, and `normal` behavior are global because there is no resolved service or mux context when those effects are applied. Supplying `service_id` or `mux_id` with a global scenario returns `400 Bad Request`.

`epg_gap` also accepts `duration_min`; if omitted, the gap is 60 minutes from the lab clock.

## `POST /api/reset`

Clears sessions and tuner state, then records a `reset` event.

This endpoint clears the lab model, active RTSP sessions, and active RTP senders owned by the simulator process. It does **not** change the active runtime scenario; use `POST /api/scenario` for that.
