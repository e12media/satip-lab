package compatibility

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

type TraceEvidence struct {
	Profile    string              `json:"profile"`
	Confidence string              `json:"confidence"`
	Source     TraceEvidenceSource `json:"source"`
	Observed   ObservedBehavior    `json:"observed"`
	Simulator  SimulatorEvidence   `json:"simulator"`
}

type TraceEvidenceSource struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ObservedBehavior struct {
	RTSPMethods                 []string `json:"rtsp_methods"`
	SessionHeader               string   `json:"session_header"`
	TransportHeader             string   `json:"transport_header"`
	SessionIDFormat             string   `json:"session_id_format"`
	SetupIncludesTimeout        *bool    `json:"setup_includes_timeout"`
	RequiresDescribeBeforeSetup *bool    `json:"requires_describe_before_setup"`
	TunerBusyStatus             string   `json:"tuner_busy_status"`
	NoSignalStatus              string   `json:"no_signal_status"`
	TimingNotes                 string   `json:"timing_notes"`
}

type SimulatorEvidence struct {
	ImplementedBehavior []string `json:"implemented_behavior"`
	Notes               string   `json:"notes"`
}

func ValidateTraceEvidence(body []byte) error {
	doc, err := parseTraceEvidence(body)
	if err != nil {
		return err
	}
	var missing []string
	require(&missing, doc.Profile, "profile")
	require(&missing, doc.Confidence, "confidence")
	require(&missing, doc.Source.Label, "source.label")
	require(&missing, doc.Source.URL, "source.url")
	if len(doc.Observed.RTSPMethods) == 0 {
		missing = append(missing, "observed.rtsp_methods")
	}
	require(&missing, doc.Observed.SessionHeader, "observed.session_header")
	require(&missing, doc.Observed.TransportHeader, "observed.transport_header")
	require(&missing, doc.Observed.SessionIDFormat, "observed.session_id_format")
	if doc.Observed.SetupIncludesTimeout == nil {
		missing = append(missing, "observed.setup_includes_timeout")
	}
	if doc.Observed.RequiresDescribeBeforeSetup == nil {
		missing = append(missing, "observed.requires_describe_before_setup")
	}
	require(&missing, doc.Observed.TunerBusyStatus, "observed.tuner_busy_status")
	require(&missing, doc.Observed.NoSignalStatus, "observed.no_signal_status")
	require(&missing, doc.Observed.TimingNotes, "observed.timing_notes")
	if !jsonFieldPresent(body, "simulator", "implemented_behavior") {
		missing = append(missing, "simulator.implemented_behavior")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required trace evidence fields: %s", strings.Join(missing, ", "))
	}

	behavior := behaviorFromObserved(doc.Observed)
	if behavior.isNonSpec() && !traceBacked(doc.Confidence) {
		return fmt.Errorf("behavior requires captured-trace or owned-hardware confidence")
	}
	return nil
}

func TraceEvidenceBehaviorYAML(body []byte) ([]byte, error) {
	doc, err := parseAndValidateTraceEvidence(body)
	if err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(map[string]behaviorDocument{
		"behavior": behaviorFromObserved(doc.Observed),
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func CheckProfileBehaviorAgainstTrace(profileYAML []byte, traceJSON []byte) error {
	trace, err := parseAndValidateTraceEvidence(traceJSON)
	if err != nil {
		return err
	}
	var profile profileDocument
	if err := yaml.Unmarshal(profileYAML, &profile); err != nil {
		return err
	}
	if strings.TrimSpace(profile.Profile) != "" && profile.Profile != trace.Profile {
		return fmt.Errorf("profile mismatch: YAML profile %q, trace profile %q", profile.Profile, trace.Profile)
	}
	if profile.Behavior == nil {
		return fmt.Errorf("profile YAML missing behavior")
	}
	expected := behaviorFromObserved(trace.Observed)
	if !reflect.DeepEqual(*profile.Behavior, expected) {
		return fmt.Errorf("behavior mismatch between profile YAML and trace evidence")
	}
	if err := ValidateProfile(profileYAML); err != nil {
		return err
	}
	return nil
}

func parseAndValidateTraceEvidence(body []byte) (TraceEvidence, error) {
	if err := ValidateTraceEvidence(body); err != nil {
		return TraceEvidence{}, err
	}
	return parseTraceEvidence(body)
}

func parseTraceEvidence(body []byte) (TraceEvidence, error) {
	var doc TraceEvidence
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return TraceEvidence{}, err
	}
	return doc, nil
}

func behaviorFromObserved(observed ObservedBehavior) behaviorDocument {
	return behaviorDocument{
		RTSPMethods:                 append([]string(nil), observed.RTSPMethods...),
		SessionHeader:               observed.SessionHeader,
		TransportHeader:             observed.TransportHeader,
		SessionIDFormat:             observed.SessionIDFormat,
		SetupIncludesTimeout:        observed.SetupIncludesTimeout,
		RequiresDescribeBeforeSetup: observed.RequiresDescribeBeforeSetup,
		TunerBusyStatus:             observed.TunerBusyStatus,
		NoSignalStatus:              observed.NoSignalStatus,
		TimingNotes:                 observed.TimingNotes,
	}
}

func jsonFieldPresent(body []byte, path ...string) bool {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	current := raw
	for _, part := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return false
		}
		next, ok := obj[part]
		if !ok {
			return false
		}
		current = next
	}
	return true
}
