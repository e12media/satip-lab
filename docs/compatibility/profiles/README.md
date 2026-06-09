# Compatibility Profile YAML

These YAML files are the public compatibility corpus. They document what is
known about each SAT>IP server profile and separate evidence from simulator
behavior.

The runtime profile registry currently ships curated built-in profiles. The YAML
files are review artifacts and future import candidates; do not add non-spec
runtime quirks from YAML unless the profile confidence is `captured-trace` or
`owned-hardware`.

Required shape:

```yaml
profile: example-profile
model:
  manufacturer: Example
  name: Example SAT>IP
source:
  - label: Public manual
    url: https://example.invalid/manual
confidence: public-doc
desc_xml:
  friendlyName: Example SAT>IP
  manufacturer: Example
  modelName: Example SAT>IP
  modelNumber: ""
  udn: ""
  description_path: /desc.xml
  x_satipcap: ""
  x_satipm3u: /channels.m3u
ssdp_headers:
  SERVER: ""
  ST: urn:ses-com:device:SatIPServer:1
  USN: ""
m3u_sample: ""
rtsp_transcript: ""
known_quirks: []
supported_methods:
  - OPTIONS
  - DESCRIBE
  - SETUP
  - PLAY
  - PAUSE
  - TEARDOWN
  - GET_PARAMETER
tuner_behavior: ""
playback_notes: ""
simulator:
  runtime_profile: example-profile
  implemented_scope: metadata-only
```

## Optional behavior evidence

Profiles may include a `behavior` section only when observations are backed by
`confidence: captured-trace` or `confidence: owned-hardware`. Metadata-only
profiles with `spec` or `public-doc` confidence must not add non-spec behavior
fields.

```yaml
behavior:
  rtsp_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN]
  session_header: Session
  transport_header: Transport
  session_id_format: numeric
  setup_includes_timeout: true
  requires_describe_before_setup: false
  tuner_busy_status: 503 Service Unavailable
  no_signal_status: 503 Service Unavailable
  timing_notes: SETUP, PLAY, and TEARDOWN timings summarized from the trace.
```

Behavior fields describe observed hardware or server behavior. They do not make
the simulator load YAML-defined quirks at runtime. Runtime behavior is still
promoted manually into Go profiles after maintainers review the trace evidence.

## Validation

The checked-in corpus is validated by `go test ./...`. The validator requires
the metadata fields above and rejects non-spec behavior on profiles that do not
have trace-level confidence.

To collect reviewable smoke evidence from a running lab server:

```bash
go run ./cmd/satip-lab-smoke --json --profile tvheadend
```

Attach the JSON output, a sanitized RTSP transcript, or a small pcap-derived
excerpt when proposing a profile confidence upgrade.

## Capture-backed ingestion workflow

Use `satip-lab-compat-evidence` for sanitized trace summaries before promoting a
metadata-only profile to behavior-backed evidence:

```bash
go run ./cmd/satip-lab-compat-evidence \
  --input docs/compatibility/evidence/example-trace.json

go run ./cmd/satip-lab-compat-evidence \
  --input docs/compatibility/evidence/example-trace.json \
  --behavior-yaml
```

The trace summary separates `observed` facts from `simulator.implemented_behavior`.
Observed fields can generate a reviewed `behavior:` YAML block, but the simulator
does not load YAML-defined behavior at runtime in this slice.

When a profile already includes behavior evidence, check it against the sanitized
summary:

```bash
go run ./cmd/satip-lab-compat-evidence \
  --input docs/compatibility/evidence/example-trace.json \
  --profile-yaml docs/compatibility/profiles/example.yaml
```

The tool rejects missing evidence metadata with field-specific errors and rejects
non-spec behavior unless `confidence` is `captured-trace` or `owned-hardware`.
