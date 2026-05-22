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
