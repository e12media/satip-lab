package httpserver

import (
	"fmt"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/lab"
	"github.com/e12media/satip-lab/internal/vendorprofile"
)

const AgentContextVersion = "1.0"

type AgentContext struct {
	Version           string                 `json:"version"`
	URLs              AgentContextURLs       `json:"urls"`
	TestEnv           map[string]string      `json:"test_env"`
	Catalog           AgentContextCatalog    `json:"catalog"`
	Features          map[string]bool        `json:"features"`
	Runtime           AgentContextRuntime    `json:"runtime"`
	Compatibility     AgentContextCompat     `json:"compatibility"`
	Scenarios         []AgentContextScenario `json:"scenarios"`
	Docs              []AgentContextDoc      `json:"docs"`
	RecommendedChecks []string               `json:"recommended_checks"`
}

type AgentContextURLs struct {
	HTTPBaseURL  string `json:"http_base_url"`
	RTSPBaseURL  string `json:"rtsp_base_url"`
	DeviceXML    string `json:"device_xml"`
	M3U          string `json:"m3u"`
	XMLTV        string `json:"xmltv"`
	Schema       string `json:"schema"`
	ConfigSchema string `json:"config_schema"`
	Status       string `json:"status"`
}

type AgentContextCatalog struct {
	ServiceCount  int    `json:"service_count"`
	MuxCount      int    `json:"mux_count"`
	Source        string `json:"source"`
	CatalogPath   string `json:"catalog_path,omitempty"`
	FixturePath   string `json:"fixture_path"`
	SampleService string `json:"sample_service"`
	SampleRTSP    string `json:"sample_rtsp_url"`
}

type AgentContextRuntime struct {
	Tuners        int    `json:"tuners"`
	Scenario      string `json:"scenario"`
	Profile       string `json:"profile"`
	ReadinessPath string `json:"readiness_path"`
	ResetPath     string `json:"reset_path"`
	ScenarioPath  string `json:"scenario_path"`
}

type AgentContextCompat struct {
	ActiveProfile     string   `json:"active_profile"`
	AvailableProfiles []string `json:"available_profiles"`
	CorpusPath        string   `json:"corpus_path"`
}

type AgentContextScenario struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	SupportsTarget bool   `json:"supports_target"`
}

type AgentContextDoc struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func buildAgentContext(cfg config.Config, manager *lab.Manager) AgentContext {
	httpBaseURL := cfg.HTTPBaseURL()
	rtspBaseURL := fmt.Sprintf("rtsp://%s:%d/", cfg.PublicHost, cfg.EffectivePublicRTSPPort())
	catalog := manager.Catalog()
	status := manager.Status()
	sampleName := ""
	sampleRTSP := ""
	if len(catalog.Channels()) > 0 {
		ch := catalog.Channels()[0]
		sampleName = ch.Name
		sampleRTSP = rtspBaseURL + "?" + ch.TuningQuery()
	}
	catalogSource := "built_in"
	if cfg.CatalogPath != "" {
		catalogSource = "yaml"
	}

	return AgentContext{
		Version: AgentContextVersion,
		URLs: AgentContextURLs{
			HTTPBaseURL:  httpBaseURL,
			RTSPBaseURL:  rtspBaseURL,
			DeviceXML:    cfg.DeviceDescriptionURL(),
			M3U:          cfg.M3UURL(),
			XMLTV:        httpBaseURL + "/epg/xmltv.xml",
			Schema:       httpBaseURL + "/api/schema",
			ConfigSchema: httpBaseURL + "/api/config/schema",
			Status:       httpBaseURL + "/api/status",
		},
		TestEnv: map[string]string{
			"SATIP_TEST_HTTP_URL": httpBaseURL,
			"SATIP_TEST_RTSP_URL": rtspBaseURL,
		},
		Catalog: AgentContextCatalog{
			ServiceCount:  len(catalog.Services),
			MuxCount:      len(catalog.Muxes),
			Source:        catalogSource,
			CatalogPath:   cfg.CatalogPath,
			FixturePath:   "fixtures/astra-19.2e-dach.yaml",
			SampleService: sampleName,
			SampleRTSP:    sampleRTSP,
		},
		Features: map[string]bool{
			"custom_catalogs":        true,
			"compatibility_profiles": true,
			"xmltv_epg":              true,
			"eit_present_following":  true,
			"rtsp_rtp_smoke":         true,
			"runtime_scenarios":      true,
		},
		Runtime: AgentContextRuntime{
			Tuners:        len(status.Tuners),
			Scenario:      manager.Scenario().Name,
			Profile:       cfg.CompatibilityProfile().Name,
			ReadinessPath: "/api/agent/context",
			ResetPath:     "/api/reset",
			ScenarioPath:  "/api/scenario",
		},
		Compatibility: AgentContextCompat{
			ActiveProfile:     cfg.CompatibilityProfile().Name,
			AvailableProfiles: vendorprofile.Names(),
			CorpusPath:        "docs/compatibility/servers.md",
		},
		Scenarios:         buildAgentScenarios(),
		Docs:              agentDocs(),
		RecommendedChecks: recommendedAgentChecks(),
	}
}

func buildAgentScenarios() []AgentContextScenario {
	scenarios := lab.SupportedScenarios()
	out := make([]AgentContextScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		out = append(out, AgentContextScenario{
			Name:           scenario.Name,
			Description:    scenario.Description,
			SupportsTarget: scenario.SupportsTarget(),
		})
	}
	return out
}

func agentDocs() []AgentContextDoc {
	return []AgentContextDoc{
		{Name: "Agent guide", Path: "docs/agents/README.md"},
		{Name: "Codex instructions", Path: "docs/agents/codex.md"},
		{Name: "Claude instructions", Path: "docs/agents/claude.md"},
		{Name: "Cursor instructions", Path: "docs/agents/cursor.md"},
		{Name: "Gemini instructions", Path: "docs/agents/gemini.md"},
		{Name: "Agent playbook", Path: "docs/agent-playbook.md"},
		{Name: "Catalogs", Path: "docs/catalog.md"},
		{Name: "Compatibility corpus", Path: "docs/compatibility/servers.md"},
		{Name: "Supported profile", Path: "docs/supported-profile.md"},
		{Name: "CI integration", Path: "docs/ci-integration.md"},
	}
}

func recommendedAgentChecks() []string {
	return []string{
		"Poll /api/agent/context or /desc.xml before client tests.",
		"Use SATIP_TEST_HTTP_URL and SATIP_TEST_RTSP_URL instead of hard-coded ports.",
		"Call POST /api/reset between independent integration tests.",
		"Use POST /api/scenario to exercise failure handling, then restore normal.",
		"Assert M3U, XMLTV, EIT p/f, RTSP setup, PLAY, and RTP behavior separately.",
		"Update docs/agents and /api/agent/context whenever new lab capabilities, config, scenarios, or companion tools are added.",
	}
}
