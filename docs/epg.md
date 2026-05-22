# XMLTV EPG Contract

`satip-lab` v1.1 exposes a deterministic XMLTV fixture at:

```text
GET /epg/xmltv.xml
```

The fixture is designed for SAT>IP client EPG parsing, channel mapping, Now/Next views, and freshness diagnostics. It is not generated from real broadcast feeds.

## Clock

The default EPG clock is fixed:

```text
SATIP_LAB_EPG_CLOCK=fixed:2026-03-29T01:30:00+01:00
```

That instant is in `Europe/Berlin` immediately before the 2026 spring-forward DST transition, so the default 24-hour XMLTV window always crosses from `+0100` to `+0200`.

Supported values:

| Value | Behavior |
|-------|----------|
| `fixed:<rfc3339>` | Deterministic lab clock. This is the default and should be used in CI. |
| `real` | Use the current wall clock in `Europe/Berlin`. This is for demos, not reproducible tests. |

The active clock is available from:

```text
GET /api/clock
```

Example:

```json
{
  "mode": "fixed",
  "now": "2026-03-29T01:30:00+01:00",
  "tz": "Europe/Berlin"
}
```

## XMLTV fields

Timestamps in XMLTV bodies use standard XMLTV format:

```text
YYYYMMDDhhmmss +ZZZZ
```

Example:

```xml
<programme start="20260329013000 +0100" stop="20260329030000 +0200" channel="zdf.de">
```

Channel identity follows the active M3U catalog. XMLTV `channel id` equals the catalog/M3U `tvg-id`, and XMLTV `display-name` equals the catalog/M3U `tvg-name`. With the default built-in catalog, the stable identities are:

| Service | XMLTV `channel id` | XMLTV `display-name` |
|---------|---------------------|----------------------|
| Das Erste HD | `daserste.de` | `Das Erste HD` |
| ZDF HD | `zdf.de` | `ZDF HD` |
| arte HD | `arte.de` | `arte HD` |
| 3sat HD | `3sat.de` | `3sat HD` |
| phoenix HD | `phoenix.de` | `phoenix HD` |

Each programme includes an `episode-num system="satip-lab"` value derived from channel id and slot start time. Programme identity is stable for the same clock and schedule slot.

## Schedule density

The generated 24-hour window intentionally mixes densities:

| Service | Slot length |
|---------|-------------|
| ZDF HD | 30 minutes |
| arte HD | 45 minutes |
| Das Erste HD | 60 minutes |
| phoenix HD | 90 minutes |
| 3sat HD | 120 minutes |

This gives clients realistic mapping and list-density coverage without requiring a large catalog. When `SATIP_LAB_CATALOG` is set, the same density pattern is assigned by service id where known and by deterministic fallback for additional services.

## Freshness

`/epg/xmltv.xml` sets an HTTP `Last-Modified` header. Under normal behavior it equals the lab clock. Under `epg_stale`, it is exactly 48 hours before the lab clock. Staleness is always relative to `GET /api/clock`, not the host wall clock.

## In-stream EIT

Generated synthetic MPEG-TS also carries minimal DVB EIT present/following on PID `0x0012`. EIT p/f uses the same lab clock and schedule density as XMLTV, but timestamps are encoded as DVB UTC MJD/BCD values inside the TS section.

The EIT scope is intentionally small:

- Table id `0x4E` for actual transport stream present/following.
- Section `0` is present, section `1` is following.
- Short event descriptor contains the deterministic programme title.
- `epg_gap` suppresses EIT p/f for the targeted service or mux.
- `epg_mismatch` is XMLTV-only in v1.5.
- `epg_stale` affects HTTP freshness only.
- `SATIP_LAB_TS_PATH` and decodable sample profiles are served unchanged.

## Scenarios

EPG scenarios are controlled through `POST /api/scenario`.

### `epg_gap`

Removes programmes for a targeted service or mux from the lab clock through a deterministic duration.
For generated synthetic TS, this also suppresses EIT p/f for the targeted service or mux while RTP media packets continue.

Example:

```json
{
  "name": "epg_gap",
  "service_id": "arte-hd",
  "duration_min": 60
}
```

If `duration_min` is omitted, the gap is 60 minutes. This scenario supports `service_id` and `mux_id` targeting.

### `epg_mismatch`

Returns XMLTV where the ZDF HD channel id is deliberately changed from `zdf.de` to `zdf-mismatch.invalid`. Programmes for that service reference the mismatched id too.

This v1.1 mismatch case tests the "XMLTV channel id not present in M3U tvg-id" path. Display-name mismatch can be added later if a client needs that separate failure mode.

### `epg_stale`

Returns normal XMLTV content but sets `Last-Modified` to 48 hours before the lab clock.
