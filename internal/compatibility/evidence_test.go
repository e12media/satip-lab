package compatibility

import (
	"strings"
	"testing"
)

func TestValidateTraceEvidenceAcceptsCapturedTrace(t *testing.T) {
	if err := ValidateTraceEvidence(validTraceEvidence()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTraceEvidenceReportsMissingMetadata(t *testing.T) {
	err := ValidateTraceEvidence([]byte(`{"profile":"tvheadend","observed":{}}`))
	if err == nil {
		t.Fatal("expected missing metadata error")
	}
	for _, field := range []string{
		"confidence",
		"source.label",
		"observed.rtsp_methods",
		"observed.session_header",
		"simulator.implemented_behavior",
	} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("expected missing %s error, got %v", field, err)
		}
	}
}

func TestValidateTraceEvidenceRejectsNonSpecWithoutTraceConfidence(t *testing.T) {
	body := strings.Replace(string(validTraceEvidence()), `"confidence": "captured-trace"`, `"confidence": "public-doc"`, 1)
	err := ValidateTraceEvidence([]byte(body))
	if err == nil || !strings.Contains(err.Error(), "behavior requires captured-trace or owned-hardware confidence") {
		t.Fatalf("expected trace confidence error, got %v", err)
	}
}

func TestTraceEvidenceBehaviorYAMLCanValidateProfileBehavior(t *testing.T) {
	behaviorYAML, err := TraceEvidenceBehaviorYAML(validTraceEvidence())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(behaviorYAML), "behavior:") || !strings.Contains(string(behaviorYAML), "session_header: session") {
		t.Fatalf("behavior YAML:\n%s", behaviorYAML)
	}

	profile := []byte(`
profile: traced-example
model:
  manufacturer: Example
  name: Example SAT>IP
source:
  - label: Sanitized trace
    url: traces/traced-example.json
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
rtsp_transcript: Sanitized trace summary in traces/traced-example.json
known_quirks:
  - Lower-case session header in PLAY responses.
supported_methods: [OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN]
tuner_behavior: Busy returns 453.
playback_notes: RTP starts after PLAY.
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
  implemented_scope: metadata-only
`)

	if err := CheckProfileBehaviorAgainstTrace(profile, validTraceEvidence()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckProfileBehaviorAgainstTraceReportsMismatch(t *testing.T) {
	profile := []byte(`
profile: traced-example
confidence: captured-trace
behavior:
  rtsp_methods: [OPTIONS]
  session_header: Session
  transport_header: Transport
  session_id_format: numeric
  setup_includes_timeout: true
  requires_describe_before_setup: false
  tuner_busy_status: 503 Service Unavailable
  no_signal_status: 503 Service Unavailable
  timing_notes: spec behavior
`)

	err := CheckProfileBehaviorAgainstTrace(profile, validTraceEvidence())
	if err == nil || !strings.Contains(err.Error(), "behavior mismatch") {
		t.Fatalf("expected behavior mismatch, got %v", err)
	}
}

func validTraceEvidence() []byte {
	return []byte(`{
  "profile": "traced-example",
  "confidence": "captured-trace",
  "source": {
    "label": "Sanitized trace",
    "url": "traces/traced-example.json"
  },
  "observed": {
    "rtsp_methods": ["OPTIONS", "DESCRIBE", "SETUP", "PLAY", "TEARDOWN"],
    "session_header": "session",
    "transport_header": "Transport",
    "session_id_format": "numeric",
    "setup_includes_timeout": false,
    "requires_describe_before_setup": true,
    "tuner_busy_status": "453 Not Enough Bandwidth",
    "no_signal_status": "503 Service Unavailable",
    "timing_notes": "SETUP response observed after 120 ms."
  },
  "simulator": {
    "implemented_behavior": [],
    "notes": "Observed behavior only; no runtime promotion in this slice."
  }
}`)
}
