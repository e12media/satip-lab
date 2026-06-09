package config_test

import (
	"testing"

	"github.com/e12media/satip-lab/internal/config"
)

func TestFromEnvironment(t *testing.T) {
	t.Setenv("SATIP_LAB_PUBLIC_HOST", "10.0.0.8")
	t.Setenv("SATIP_LAB_HTTP_PORT", "18080")
	t.Setenv("SATIP_LAB_RTSP_PORT", "1554")
	t.Setenv("SATIP_LAB_PUBLIC_HTTP_PORT", "28080")
	t.Setenv("SATIP_LAB_PUBLIC_RTSP_PORT", "2554")
	t.Setenv("SATIP_LAB_TUNERS", "4")
	t.Setenv("SATIP_LAB_CATALOG", "/tmp/channels.yaml")
	t.Setenv("SATIP_LAB_TOPOLOGY", "/tmp/topology.yaml")
	t.Setenv("SATIP_LAB_SAMPLE_PROFILE", "h264_silent")
	t.Setenv("SATIP_LAB_PROFILE", "minisatip")
	t.Setenv("SATIP_LAB_VENDOR_PROFILE", "spec")
	t.Setenv("SATIP_LAB_EPG_CLOCK", "real")
	t.Setenv("SATIP_LAB_SCENARIO", "tuner_busy")

	cfg := config.FromEnvironment()
	if cfg.PublicHost != "10.0.0.8" {
		t.Fatalf("public host: got %q", cfg.PublicHost)
	}
	if cfg.HTTPPort != 18080 || cfg.RTSPPort != 1554 {
		t.Fatalf("ports: http=%d rtsp=%d", cfg.HTTPPort, cfg.RTSPPort)
	}
	if cfg.PublicHTTPPort != 28080 || cfg.PublicRTSPPort != 2554 {
		t.Fatalf("public ports: http=%d rtsp=%d", cfg.PublicHTTPPort, cfg.PublicRTSPPort)
	}
	if cfg.TunerCount != 4 {
		t.Fatalf("tuners: got %d", cfg.TunerCount)
	}
	if cfg.CatalogPath != "/tmp/channels.yaml" {
		t.Fatalf("catalog path: got %q", cfg.CatalogPath)
	}
	if cfg.TopologyPath != "/tmp/topology.yaml" {
		t.Fatalf("topology path: got %q", cfg.TopologyPath)
	}
	if cfg.EPGClock != "real" {
		t.Fatalf("epg clock: got %q", cfg.EPGClock)
	}
	if cfg.SampleProfile != "h264_silent" {
		t.Fatalf("sample profile: got %q", cfg.SampleProfile)
	}
	if cfg.Profile != "minisatip" {
		t.Fatalf("profile: got %q", cfg.Profile)
	}
	if cfg.VendorProfile != "spec" {
		t.Fatalf("vendor profile: got %q", cfg.VendorProfile)
	}
	if got := cfg.DeviceDescriptionURL(); got != "http://10.0.0.8:28080/desc.xml" {
		t.Fatalf("device URL: got %q", got)
	}
	if cfg.Scenario != config.ScenarioTunerBusy {
		t.Fatal("expected tuner_busy scenario")
	}
}

func TestFromEnvironmentIgnoresLegacySatipSimVariables(t *testing.T) {
	legacyPrefix := "SATIP" + "_" + "SIM" + "_"
	t.Setenv(legacyPrefix+"PUBLIC_HOST", "10.0.0.8")
	t.Setenv(legacyPrefix+"HTTP_PORT", "18080")
	t.Setenv(legacyPrefix+"RTSP_PORT", "1554")
	t.Setenv(legacyPrefix+"TUNERS", "4")
	t.Setenv(legacyPrefix+"PROFILE", "minisatip")
	t.Setenv(legacyPrefix+"SCENARIO", "tuner_busy")

	cfg := config.FromEnvironment()
	if cfg.PublicHost != "127.0.0.1" {
		t.Fatalf("public host should ignore legacy variable, got %q", cfg.PublicHost)
	}
	if cfg.HTTPPort != 8875 || cfg.RTSPPort != 554 {
		t.Fatalf("ports should ignore legacy variables: http=%d rtsp=%d", cfg.HTTPPort, cfg.RTSPPort)
	}
	if cfg.TunerCount != 2 {
		t.Fatalf("tuners should ignore legacy variable, got %d", cfg.TunerCount)
	}
	if cfg.Profile != "generic-satip-1.2" {
		t.Fatalf("profile should ignore legacy variable, got %q", cfg.Profile)
	}
	if cfg.Scenario != config.ScenarioNormal {
		t.Fatal("scenario should ignore legacy variable")
	}
}

func TestPublicPortsFallBackToListenPorts(t *testing.T) {
	cfg := config.Config{
		PublicHost: "127.0.0.1",
		HTTPPort:   8875,
		RTSPPort:   554,
	}

	if cfg.EffectivePublicHTTPPort() != 8875 {
		t.Fatalf("public HTTP port: got %d", cfg.EffectivePublicHTTPPort())
	}
	if cfg.EffectivePublicRTSPPort() != 554 {
		t.Fatalf("public RTSP port: got %d", cfg.EffectivePublicRTSPPort())
	}
}

func TestProfileCanChangeDeviceDescriptionURL(t *testing.T) {
	cfg := config.Config{
		PublicHost:     "satip.test",
		PublicHTTPPort: 18875,
		Profile:        "tvheadend",
	}

	if got := cfg.DeviceDescriptionURL(); got != "http://satip.test:18875/satip_server/desc.xml" {
		t.Fatalf("device description URL: got %q", got)
	}
}

func TestSchemaListsStableEnvironmentContract(t *testing.T) {
	schema := config.Schema()

	if schema.Version != "2.1" {
		t.Fatalf("schema version: got %q", schema.Version)
	}
	if len(schema.Variables) != 16 {
		t.Fatalf("schema variables: got %d", len(schema.Variables))
	}
	if schema.Variables[0].Name != "SATIP_LAB_BIND" || schema.Variables[0].Default != "0.0.0.0" {
		t.Fatalf("first schema entry: %+v", schema.Variables[0])
	}
	if schema.Variables[len(schema.Variables)-1].Name != "SATIP_LAB_SCENARIO" {
		t.Fatalf("last schema entry: %+v", schema.Variables[len(schema.Variables)-1])
	}
	foundEPGClock := false
	foundSampleProfile := false
	foundProfile := false
	foundVendorProfile := false
	foundCatalog := false
	foundTopology := false
	for _, variable := range schema.Variables {
		if variable.Name == "SATIP_LAB_CATALOG" {
			foundCatalog = true
			if variable.Default != "" {
				t.Fatalf("catalog default: %+v", variable)
			}
		}
		if variable.Name == "SATIP_LAB_TOPOLOGY" {
			foundTopology = true
			if variable.Default != "" {
				t.Fatalf("topology default: %+v", variable)
			}
		}
		if variable.Name == "SATIP_LAB_SAMPLE_PROFILE" {
			foundSampleProfile = true
			if variable.Default != "synthetic" {
				t.Fatalf("sample profile default: %+v", variable)
			}
			if !sameStrings(variable.Enum, []string{"synthetic", "h264_aac_short", "h264_silent"}) {
				t.Fatalf("sample profile enum: %+v", variable)
			}
		}
		if variable.Name == "SATIP_LAB_PROFILE" {
			foundProfile = true
			if variable.Default != "generic-satip-1.2" {
				t.Fatalf("profile default: %+v", variable)
			}
			if len(variable.Enum) < 3 {
				t.Fatalf("profile enum: %+v", variable)
			}
		}
		if variable.Name == "SATIP_LAB_EPG_CLOCK" {
			foundEPGClock = true
			if variable.Default != "fixed:2026-03-29T01:30:00+01:00" {
				t.Fatalf("EPG clock default: %+v", variable)
			}
		}
		if variable.Name == "SATIP_LAB_VENDOR_PROFILE" {
			foundVendorProfile = true
			if variable.Default != "spec" {
				t.Fatalf("vendor profile default: %+v", variable)
			}
			if len(variable.Enum) < 3 {
				t.Fatalf("vendor profile enum: %+v", variable)
			}
		}
	}
	if !foundEPGClock {
		t.Fatal("missing SATIP_LAB_EPG_CLOCK schema entry")
	}
	if !foundSampleProfile {
		t.Fatal("missing SATIP_LAB_SAMPLE_PROFILE schema entry")
	}
	if !foundProfile {
		t.Fatal("missing SATIP_LAB_PROFILE schema entry")
	}
	if !foundVendorProfile {
		t.Fatal("missing SATIP_LAB_VENDOR_PROFILE schema entry")
	}
	if !foundCatalog {
		t.Fatal("missing SATIP_LAB_CATALOG schema entry")
	}
	if !foundTopology {
		t.Fatal("missing SATIP_LAB_TOPOLOGY schema entry")
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
