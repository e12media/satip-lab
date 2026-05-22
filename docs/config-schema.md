# Configuration Schema

Schema version: **2.0**

This is the stable v1 configuration contract for `satip-lab`. The same custom schema document is available at runtime as JSON from `GET /api/config/schema`; it is not a JSON Schema draft document.

| Variable | Default | Type | Description |
|----------|---------|------|-------------|
| `SATIP_LAB_BIND` | `0.0.0.0` | string | Listen address for HTTP, RTSP, and SSDP sockets. |
| `SATIP_LAB_PUBLIC_HOST` | `127.0.0.1` | string | Host advertised in SSDP `LOCATION` and generated SAT>IP URLs. |
| `SATIP_LAB_HTTP_PORT` | `8875` | integer | HTTP listen port for `desc.xml`, M3U, status, and lab API. |
| `SATIP_LAB_RTSP_PORT` | `554` | integer | RTSP listen port. |
| `SATIP_LAB_PUBLIC_HTTP_PORT` | `0` | integer | Advertised HTTP port; `0` uses `SATIP_LAB_HTTP_PORT`. |
| `SATIP_LAB_PUBLIC_RTSP_PORT` | `0` | integer | Advertised RTSP port in M3U URLs; `0` uses `SATIP_LAB_RTSP_PORT`. |
| `SATIP_LAB_TUNERS` | `2` | integer | Synthetic SAT>IP tuner count. |
| `SATIP_LAB_SSDP_PORT` | `1900` | integer | SSDP UDP port; `0` disables SSDP. |
| `SATIP_LAB_CATALOG` | empty | string | Optional YAML channel catalog path; empty uses the built-in five-service DACH catalog. |
| `SATIP_LAB_TS_PATH` | empty | string | Optional MPEG-TS file to loop for all services; empty uses generated TS. |
| `SATIP_LAB_SAMPLE_PROFILE` | `synthetic` | string | Built-in service sample profile used when `SATIP_LAB_TS_PATH` is empty. Values: `synthetic`, `h264_aac_short`, `h264_silent`. |
| `SATIP_LAB_PROFILE` | `generic-satip-1.2` | string | Compatibility profile for SSDP, device XML path/metadata, M3U, and RTSP behavior. Values: `generic-satip-1.2`, `spec`, `minisatip`, `tvheadend`, `triax-tss400`, `telestar-digibit-r1`, `kathrein-exip`, `digital-devices-octopus-net`. |
| `SATIP_LAB_VENDOR_PROFILE` | `spec` | string | RTSP behavior profile selector alias. `SATIP_LAB_PROFILE` is preferred. |
| `SATIP_LAB_EPG_CLOCK` | `fixed:2026-03-29T01:30:00+01:00` | string | EPG lab clock: `fixed:<rfc3339>` for deterministic XMLTV output or `real` for wall-clock demos. |
| `SATIP_LAB_SCENARIO` | `normal` | string | Startup RTSP scenario: `normal` or `tuner_busy`. |

Changing or removing a variable, default, type, or enum value requires a schema version update.
