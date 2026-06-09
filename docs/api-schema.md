# API Schema

Schema version: **1.10**

This is the stable v1 lab API contract for `satip-lab`. The same custom schema document is available at runtime as JSON from `GET /api/schema`; it is not a JSON Schema or OpenAPI document.

## Endpoints

| Path | Methods | Description |
|------|---------|-------------|
| `/api/agent/context` | `GET` | Coding agent bootstrap context with URLs, scenarios, docs, and recommended checks. |
| `/api/config/schema` | `GET` | Versioned configuration contract. |
| `/api/clock` | `GET` | Current deterministic lab clock for EPG generation. |
| `/api/schema` | `GET` | Versioned lab API contract. |
| `/api/status` | `GET` | Full lab status. |
| `/api/topology` | `GET` | Deterministic multi-device topology fixture for client tests. |
| `/api/catalog` | `GET` | Mux and service catalog. |
| `/api/muxes` | `GET` | Mux catalog entries. |
| `/api/services` | `GET` | Service catalog entries. |
| `/api/tuners` | `GET` | Simulated tuner state. |
| `/api/sessions` | `GET` | Active RTSP lab sessions. |
| `/api/playback/diagnostics` | `GET` | Per-session playback diagnostics summary. |
| `/api/events` | `GET` | Recent lab events. |
| `/api/scenario` | `GET`, `POST` | Runtime scenario state and switching. |
| `/api/reset` | `POST` | Reset lab sessions and tuner state. |
| `/epg/xmltv.xml` | `GET` | Deterministic XMLTV EPG for the lab catalog. |

## Models

The runtime schema lists stable top-level JSON field names for `agent_context`, `clock`, `catalog`, `status`, `hardware_status`, `hardware_identity`, `hardware_streams`, `hardware_tuners`, `hardware_network`, `topology`, `topology_device`, `tuner`, `frontend`, `session`, `playback_diagnostic`, `event`, `scenario`, `scenario_timeline`, `scenario_timeline_step`, `mux`, and `service` models.

The `session` model includes playback observability fields for RTSP setup/play
acceptance, RTP first/last send timestamps, packet and byte counters, transport,
and destination.

The `playback_diagnostic` model summarizes active sessions with service,
scenario, RTP destination, exact UDP ports or interleaved channels, RTP timing,
observed packet rate after first RTP, continuity-error state, optional exact
continuity-error count, and intentional impairment flags.

Changing or removing an endpoint, method, model name, or field requires a schema version update.
