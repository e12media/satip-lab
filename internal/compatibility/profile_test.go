package compatibility

import (
	"strings"
	"testing"
)

func TestValidateCheckedInProfiles(t *testing.T) {
	if err := ValidateProfileDir("../../docs/compatibility/profiles"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateProfileRejectsPublicDocNonSpecBehavior(t *testing.T) {
	doc := []byte(`
profile: example
model:
  manufacturer: Example
  name: Example SAT>IP
source:
  - label: Manual
    url: https://example.invalid/manual
confidence: public-doc
desc_xml:
  friendlyName: Example
  manufacturer: Example
  modelName: Example SAT>IP
  modelNumber: ""
  udn: uuid:example
  description_path: /desc.xml
  x_satipcap: DVBS2-{tuners}
  x_satipm3u: /channels.m3u
ssdp_headers:
  SERVER: Example UPnP/1.0
  ST: urn:ses-com:device:SatIPServer:1
  USN: uuid:example::urn:ses-com:device:SatIPServer:1
m3u_sample: /channels.m3u
rtsp_transcript: ""
known_quirks: []
supported_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER]
tuner_behavior: Not modeled.
playback_notes: Not modeled.
behavior:
  session_header: session
simulator:
  runtime_profile: example
  implemented_scope: metadata-only
`)

	err := ValidateProfile(doc)
	if err == nil || !strings.Contains(err.Error(), "behavior requires captured-trace or owned-hardware confidence") {
		t.Fatalf("expected behavior confidence error, got %v", err)
	}
}

func TestValidateProfileAllowsCapturedTraceBehavior(t *testing.T) {
	doc := []byte(`
profile: traced-example
model:
  manufacturer: Example
  name: Example SAT>IP
source:
  - label: Sanitized trace
    url: https://example.invalid/trace
confidence: captured-trace
desc_xml:
  friendlyName: Example
  manufacturer: Example
  modelName: Example SAT>IP
  modelNumber: ""
  udn: uuid:example
  description_path: /desc.xml
  x_satipcap: DVBS2-{tuners}
  x_satipm3u: /channels.m3u
ssdp_headers:
  SERVER: Example UPnP/1.0
  ST: urn:ses-com:device:SatIPServer:1
  USN: uuid:example::urn:ses-com:device:SatIPServer:1
m3u_sample: /channels.m3u
rtsp_transcript: SETUP/PLAY trace excerpt
known_quirks:
  - Lower-case session header in PLAY responses.
supported_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN]
tuner_behavior: Busy returns 453.
playback_notes: RTP starts after SETUP.
behavior:
  rtsp_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN]
  session_header: session
  transport_header: Transport
  session_id_format: numeric
  setup_includes_timeout: false
  requires_describe_before_setup: true
  tuner_busy_status: 453 Not Enough Bandwidth
  no_signal_status: 503 Service Unavailable
  timing_notes: SETUP response observed after 120 ms.
simulator:
  runtime_profile: traced-example
  implemented_scope: trace-backed-behavior
`)

	if err := ValidateProfile(doc); err != nil {
		t.Fatal(err)
	}
}

func TestValidateProfileReportsMissingMetadataFields(t *testing.T) {
	doc := []byte(`
profile: incomplete
confidence: public-doc
`)

	err := ValidateProfile(doc)
	if err == nil || !strings.Contains(err.Error(), "model.manufacturer") {
		t.Fatalf("expected missing model.manufacturer error, got %v", err)
	}
}

func TestValidateProfileRequiresEmptyCapableMetadataKeys(t *testing.T) {
	doc := []byte(`
profile: missing-empty-capable-fields
model:
  manufacturer: Example
  name: Example SAT>IP
source:
  - label: Manual
    url: https://example.invalid/manual
confidence: public-doc
desc_xml:
  friendlyName: Example
  manufacturer: Example
  modelName: Example SAT>IP
  description_path: /desc.xml
  x_satipcap: DVBS2-{tuners}
  x_satipm3u: /channels.m3u
ssdp_headers:
  SERVER: Example UPnP/1.0
  ST: urn:ses-com:device:SatIPServer:1
  USN: uuid:example::urn:ses-com:device:SatIPServer:1
supported_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER]
tuner_behavior: Not modeled.
playback_notes: Not modeled.
simulator:
  runtime_profile: example
  implemented_scope: metadata-only
`)

	err := ValidateProfile(doc)
	if err == nil {
		t.Fatal("expected missing key error")
	}
	for _, field := range []string{
		"desc_xml.modelNumber",
		"desc_xml.udn",
		"m3u_sample",
		"rtsp_transcript",
		"known_quirks",
	} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("expected missing %s error, got %v", field, err)
		}
	}
}
