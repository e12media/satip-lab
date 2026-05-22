package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/e12media/satip-lab/internal/vendorprofile"
)

const SatIPSearchTarget = "urn:ses-com:device:SatIPServer:1"
const SchemaVersion = "2.0"

type Scenario int

const (
	ScenarioNormal Scenario = iota
	ScenarioTunerBusy
)

type Config struct {
	BindAddress         string
	PublicHost          string
	HTTPPort            int
	RTSPPort            int
	PublicHTTPPort      int
	PublicRTSPPort      int
	TunerCount          int
	SSDPort             int
	CatalogPath         string
	TransportStreamPath string
	SampleProfile       string
	Profile             string
	VendorProfile       string
	EPGClock            string
	Scenario            Scenario
}

type SchemaDocument struct {
	Version   string           `json:"version"`
	Variables []SchemaVariable `json:"variables"`
}

type SchemaVariable struct {
	Name        string   `json:"name"`
	Default     string   `json:"default"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

func Schema() SchemaDocument {
	return SchemaDocument{
		Version: SchemaVersion,
		Variables: []SchemaVariable{
			{Name: "SATIP_LAB_BIND", Default: "0.0.0.0", Type: "string", Description: "Listen address for HTTP, RTSP, and SSDP sockets."},
			{Name: "SATIP_LAB_PUBLIC_HOST", Default: "127.0.0.1", Type: "string", Description: "Host advertised in SSDP LOCATION and generated SAT>IP URLs."},
			{Name: "SATIP_LAB_HTTP_PORT", Default: "8875", Type: "integer", Description: "HTTP listen port for desc.xml, M3U, status, and lab API."},
			{Name: "SATIP_LAB_RTSP_PORT", Default: "554", Type: "integer", Description: "RTSP listen port."},
			{Name: "SATIP_LAB_PUBLIC_HTTP_PORT", Default: "0", Type: "integer", Description: "Advertised HTTP port; 0 uses SATIP_LAB_HTTP_PORT."},
			{Name: "SATIP_LAB_PUBLIC_RTSP_PORT", Default: "0", Type: "integer", Description: "Advertised RTSP port in M3U URLs; 0 uses SATIP_LAB_RTSP_PORT."},
			{Name: "SATIP_LAB_TUNERS", Default: "2", Type: "integer", Description: "Synthetic SAT>IP tuner count."},
			{Name: "SATIP_LAB_SSDP_PORT", Default: "1900", Type: "integer", Description: "SSDP UDP port; 0 disables SSDP."},
			{Name: "SATIP_LAB_CATALOG", Default: "", Type: "string", Description: "Optional YAML channel catalog path; empty uses the built-in five-service DACH catalog."},
			{Name: "SATIP_LAB_TS_PATH", Default: "", Type: "string", Description: "Optional MPEG-TS file to loop for all services; empty uses generated TS."},
			{Name: "SATIP_LAB_SAMPLE_PROFILE", Default: "synthetic", Type: "string", Description: "Built-in service sample profile used when SATIP_LAB_TS_PATH is empty.", Enum: []string{"synthetic", "h264_aac_short", "h264_silent"}},
			{Name: "SATIP_LAB_PROFILE", Default: vendorprofile.NameGeneric, Type: "string", Description: "Compatibility profile for SSDP, device XML path/metadata, M3U, and RTSP behavior.", Enum: vendorprofile.Names()},
			{Name: "SATIP_LAB_VENDOR_PROFILE", Default: vendorprofile.NameSpec, Type: "string", Description: "RTSP behavior profile selector alias. SATIP_LAB_PROFILE is preferred.", Enum: vendorprofile.Names()},
			{Name: "SATIP_LAB_EPG_CLOCK", Default: "fixed:2026-03-29T01:30:00+01:00", Type: "string", Description: "EPG lab clock: fixed:<rfc3339> for deterministic XMLTV output or real for wall-clock demos."},
			{Name: "SATIP_LAB_SCENARIO", Default: "normal", Type: "string", Description: "Startup RTSP scenario.", Enum: []string{"normal", "tuner_busy"}},
		},
	}
}

func FromEnvironment() Config {
	return Config{
		BindAddress:         envOr("SATIP_LAB_BIND", "0.0.0.0"),
		PublicHost:          envOr("SATIP_LAB_PUBLIC_HOST", "127.0.0.1"),
		HTTPPort:            envInt("SATIP_LAB_HTTP_PORT", 8875),
		RTSPPort:            envInt("SATIP_LAB_RTSP_PORT", 554),
		PublicHTTPPort:      envInt("SATIP_LAB_PUBLIC_HTTP_PORT", 0),
		PublicRTSPPort:      envInt("SATIP_LAB_PUBLIC_RTSP_PORT", 0),
		TunerCount:          envInt("SATIP_LAB_TUNERS", 2),
		SSDPort:             envInt("SATIP_LAB_SSDP_PORT", 1900),
		CatalogPath:         envOr("SATIP_LAB_CATALOG", ""),
		TransportStreamPath: envOr("SATIP_LAB_TS_PATH", ""),
		SampleProfile:       envOr("SATIP_LAB_SAMPLE_PROFILE", "synthetic"),
		Profile:             envProfile(),
		VendorProfile:       envOr("SATIP_LAB_VENDOR_PROFILE", vendorprofile.NameSpec),
		EPGClock:            envOr("SATIP_LAB_EPG_CLOCK", "fixed:2026-03-29T01:30:00+01:00"),
		Scenario:            parseScenario(os.Getenv("SATIP_LAB_SCENARIO")),
	}
}

func (c Config) HTTPBaseURL() string {
	return fmt.Sprintf("http://%s:%d", c.PublicHost, c.EffectivePublicHTTPPort())
}

func (c Config) DeviceDescriptionURL() string {
	return c.HTTPBaseURL() + c.CompatibilityProfile().Device.DescriptionPath
}

func (c Config) PresentationURL() string {
	return "/"
}

func (c Config) M3UURL() string {
	return c.HTTPBaseURL() + c.CompatibilityProfile().Device.XSatipM3U
}

func (c Config) EffectivePublicHTTPPort() int {
	if c.PublicHTTPPort > 0 {
		return c.PublicHTTPPort
	}
	return c.HTTPPort
}

func (c Config) EffectivePublicRTSPPort() int {
	if c.PublicRTSPPort > 0 {
		return c.PublicRTSPPort
	}
	return c.RTSPPort
}

func (c Config) ScenarioName() string {
	if c.Scenario == ScenarioTunerBusy {
		return "tuner_busy"
	}
	return "normal"
}

func (c Config) CompatibilityProfile() vendorprofile.Profile {
	if strings.TrimSpace(c.Profile) != "" {
		return vendorprofile.ForName(c.Profile)
	}
	if strings.TrimSpace(c.VendorProfile) != "" {
		return vendorprofile.ForName(c.VendorProfile)
	}
	return vendorprofile.ForName(vendorprofile.NameGeneric)
}

func parseScenario(raw string) Scenario {
	if strings.EqualFold(raw, "tuner_busy") {
		return ScenarioTunerBusy
	}
	return ScenarioNormal
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envProfile() string {
	if v := os.Getenv("SATIP_LAB_PROFILE"); v != "" {
		return v
	}
	if v := os.Getenv("SATIP_LAB_VENDOR_PROFILE"); v != "" {
		return v
	}
	return vendorprofile.NameGeneric
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
