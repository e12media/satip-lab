package compatibility

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type profileDocument struct {
	Profile          string            `yaml:"profile"`
	Model            modelDocument     `yaml:"model"`
	Source           []sourceDocument  `yaml:"source"`
	Confidence       string            `yaml:"confidence"`
	DescXML          descXMLDocument   `yaml:"desc_xml"`
	SSDPHeaders      ssdpDocument      `yaml:"ssdp_headers"`
	M3USample        string            `yaml:"m3u_sample"`
	RTSPTranscript   string            `yaml:"rtsp_transcript"`
	KnownQuirks      []string          `yaml:"known_quirks"`
	SupportedMethods []string          `yaml:"supported_methods"`
	TunerBehavior    string            `yaml:"tuner_behavior"`
	PlaybackNotes    string            `yaml:"playback_notes"`
	Behavior         *behaviorDocument `yaml:"behavior"`
	Simulator        simulatorDocument `yaml:"simulator"`
}

type modelDocument struct {
	Manufacturer string `yaml:"manufacturer"`
	Name         string `yaml:"name"`
}

type sourceDocument struct {
	Label string `yaml:"label"`
	URL   string `yaml:"url"`
}

type descXMLDocument struct {
	FriendlyName    string `yaml:"friendlyName"`
	Manufacturer    string `yaml:"manufacturer"`
	ModelName       string `yaml:"modelName"`
	ModelNumber     string `yaml:"modelNumber"`
	UDN             string `yaml:"udn"`
	DescriptionPath string `yaml:"description_path"`
	XSatipCAP       string `yaml:"x_satipcap"`
	XSatipM3U       string `yaml:"x_satipm3u"`
}

type ssdpDocument struct {
	Server string `yaml:"SERVER"`
	ST     string `yaml:"ST"`
	USN    string `yaml:"USN"`
}

type behaviorDocument struct {
	RTSPMethods                 []string `yaml:"rtsp_methods"`
	SessionHeader               string   `yaml:"session_header"`
	TransportHeader             string   `yaml:"transport_header"`
	SessionIDFormat             string   `yaml:"session_id_format"`
	SetupIncludesTimeout        *bool    `yaml:"setup_includes_timeout"`
	RequiresDescribeBeforeSetup *bool    `yaml:"requires_describe_before_setup"`
	TunerBusyStatus             string   `yaml:"tuner_busy_status"`
	NoSignalStatus              string   `yaml:"no_signal_status"`
	TimingNotes                 string   `yaml:"timing_notes"`
}

type simulatorDocument struct {
	RuntimeProfile   string `yaml:"runtime_profile"`
	ImplementedScope string `yaml:"implemented_scope"`
}

func ValidateProfileDir(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("%s: no compatibility profile YAML files found", dir)
	}
	var failures []string
	for _, path := range matches {
		if err := ValidateProfileFile(path); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	return nil
}

func ValidateProfileFile(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := ValidateProfile(body); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func ValidateProfile(body []byte) error {
	present, err := presentFields(body)
	if err != nil {
		return err
	}
	var doc profileDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return err
	}
	var missing []string
	require(&missing, doc.Profile, "profile")
	require(&missing, doc.Model.Manufacturer, "model.manufacturer")
	require(&missing, doc.Model.Name, "model.name")
	if len(doc.Source) == 0 {
		missing = append(missing, "source")
	} else {
		require(&missing, doc.Source[0].Label, "source[0].label")
	}
	require(&missing, doc.Confidence, "confidence")
	require(&missing, doc.DescXML.FriendlyName, "desc_xml.friendlyName")
	require(&missing, doc.DescXML.Manufacturer, "desc_xml.manufacturer")
	require(&missing, doc.DescXML.ModelName, "desc_xml.modelName")
	requirePresent(&missing, present, "desc_xml.modelNumber")
	requirePresent(&missing, present, "desc_xml.udn")
	require(&missing, doc.DescXML.DescriptionPath, "desc_xml.description_path")
	require(&missing, doc.DescXML.XSatipCAP, "desc_xml.x_satipcap")
	require(&missing, doc.DescXML.XSatipM3U, "desc_xml.x_satipm3u")
	require(&missing, doc.SSDPHeaders.Server, "ssdp_headers.SERVER")
	require(&missing, doc.SSDPHeaders.ST, "ssdp_headers.ST")
	require(&missing, doc.SSDPHeaders.USN, "ssdp_headers.USN")
	requirePresent(&missing, present, "m3u_sample")
	requirePresent(&missing, present, "rtsp_transcript")
	requirePresent(&missing, present, "known_quirks")
	if len(doc.SupportedMethods) == 0 {
		missing = append(missing, "supported_methods")
	}
	require(&missing, doc.TunerBehavior, "tuner_behavior")
	require(&missing, doc.PlaybackNotes, "playback_notes")
	require(&missing, doc.Simulator.RuntimeProfile, "simulator.runtime_profile")
	require(&missing, doc.Simulator.ImplementedScope, "simulator.implemented_scope")
	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	if doc.Behavior != nil {
		if err := validateBehavior(doc.Confidence, doc.Behavior); err != nil {
			return err
		}
	}
	return nil
}

func validateBehavior(confidence string, behavior *behaviorDocument) error {
	if behavior.isNonSpec() && !traceBacked(confidence) {
		return fmt.Errorf("behavior requires captured-trace or owned-hardware confidence")
	}
	var missing []string
	if len(behavior.RTSPMethods) == 0 {
		missing = append(missing, "behavior.rtsp_methods")
	}
	require(&missing, behavior.SessionHeader, "behavior.session_header")
	require(&missing, behavior.TransportHeader, "behavior.transport_header")
	require(&missing, behavior.SessionIDFormat, "behavior.session_id_format")
	if behavior.SetupIncludesTimeout == nil {
		missing = append(missing, "behavior.setup_includes_timeout")
	}
	if behavior.RequiresDescribeBeforeSetup == nil {
		missing = append(missing, "behavior.requires_describe_before_setup")
	}
	require(&missing, behavior.TunerBusyStatus, "behavior.tuner_busy_status")
	require(&missing, behavior.NoSignalStatus, "behavior.no_signal_status")
	require(&missing, behavior.TimingNotes, "behavior.timing_notes")
	if len(missing) > 0 {
		return fmt.Errorf("missing required behavior fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (b behaviorDocument) isNonSpec() bool {
	if b.SessionHeader != "" && b.SessionHeader != "Session" {
		return true
	}
	if b.TransportHeader != "" && b.TransportHeader != "Transport" {
		return true
	}
	if b.SessionIDFormat != "" && b.SessionIDFormat != "numeric" {
		return true
	}
	if b.SetupIncludesTimeout != nil && !*b.SetupIncludesTimeout {
		return true
	}
	if b.RequiresDescribeBeforeSetup != nil && *b.RequiresDescribeBeforeSetup {
		return true
	}
	if b.TunerBusyStatus != "503 Service Unavailable" {
		return true
	}
	if b.NoSignalStatus != "503 Service Unavailable" {
		return true
	}
	return false
}

func traceBacked(confidence string) bool {
	switch strings.TrimSpace(confidence) {
	case "captured-trace", "owned-hardware":
		return true
	default:
		return false
	}
}

func presentFields(body []byte) (map[string]struct{}, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	fields := make(map[string]struct{})
	if len(root.Content) == 0 {
		return fields, nil
	}
	recordFields(fields, "", root.Content[0])
	return fields, nil
}

func recordFields(fields map[string]struct{}, prefix string, node *yaml.Node) {
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		fields[path] = struct{}{}
		recordFields(fields, path, node.Content[i+1])
	}
}

func require(missing *[]string, value, field string) {
	if strings.TrimSpace(value) == "" {
		*missing = append(*missing, field)
	}
}

func requirePresent(missing *[]string, fields map[string]struct{}, field string) {
	if _, ok := fields[field]; !ok {
		*missing = append(*missing, field)
	}
}
