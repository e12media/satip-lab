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
	Clock        string `json:"clock"`
	Schema       string `json:"schema"`
	ConfigSchema string `json:"config_schema"`
	Status       string `json:"status"`
	Topology     string `json:"topology"`
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
	Name                  string `json:"name"`
	Description           string `json:"description"`
	SupportsTarget        bool   `json:"supports_target"`
	ClientExpectationHint string `json:"client_expectation_hint,omitempty"`
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
			Clock:        httpBaseURL + "/api/clock",
			Schema:       httpBaseURL + "/api/schema",
			ConfigSchema: httpBaseURL + "/api/config/schema",
			Status:       httpBaseURL + "/api/status",
			Topology:     httpBaseURL + "/api/topology",
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
			"compatibility_evidence": true,
			"compatibility_profiles": true,
			"dvb_si_basics":          true,
			"xmltv_epg":              true,
			"eit_present_following":  true,
			"frontend_lifecycle":     true,
			"frontend_telemetry":     true,
			"hardware_status":        true,
			"multi_server_topology":  true,
			"playback_observability": true,
			"rtsp_interleaved_tcp":   true,
			"rtsp_rtp_smoke":         true,
			"runtime_scenarios":      true,
			"scenario_timelines":     true,
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
			Name:                  scenario.Name,
			Description:           scenario.Description,
			SupportsTarget:        scenario.SupportsTarget(),
			ClientExpectationHint: scenarioExpectationHint(scenario.Name),
		})
	}
	return out
}

func scenarioExpectationHint(name string) string {
	switch name {
	case lab.ScenarioTunerBusy:
		return "Valid RTSP SETUP returns 503 Service Unavailable with Reason: tuner busy; no tuner or session is allocated."
	case lab.ScenarioTunerWedged:
		return "Valid RTSP SETUP returns 503 Service Unavailable with Reason: tuner wedged until POST /api/reset clears the wedged fault."
	case lab.ScenarioColdBoot:
		return "RTSP responses are delayed by 750 ms to mimic deterministic cold-boot startup latency."
	case lab.ScenarioRTPStop:
		return "RTSP SETUP and PLAY return 200, then exactly 3 RTP packets are sent before packet delivery stops without TEARDOWN."
	case lab.ScenarioRTPBlackhole:
		return "RTSP SETUP and PLAY return 200 and the RTSP session remains alive, but all RTP packets are dropped."
	case lab.ScenarioDelayedPSI:
		return "RTSP SETUP and PLAY return 200, then the initial RTP packets carrying startup PAT/PMT evidence are delayed before normal RTP resumes."
	case lab.ScenarioRTPLoss:
		return "RTSP SETUP and PLAY return 200, then every third RTP packet is dropped; clients should report loss or recover without session setup failure."
	case lab.ScenarioRTPJitter:
		return "RTSP SETUP and PLAY return 200, then every third RTP packet is delayed by 40 ms; clients should show buffering or timing tolerance without treating SETUP as failed."
	case lab.ScenarioContinuityErrors:
		return "RTP packet framing remains valid, but MPEG-TS continuity counters are corrupted; clients should surface TS continuity errors or recover at the demux layer."
	case lab.ScenarioMalformedPSI:
		return "RTP and MPEG-TS packet framing remain valid, but PAT/PMT headers are corrupted; clients should surface PSI/parser evidence rather than transport failure."
	case lab.ScenarioSignalDegraded:
		return "RTSP SETUP and PLAY still succeed, while /api/tuners reports frontend.state=degraded with deterministic signal_strength=42, snr_db=6.5, ber=0.00025, and per=0.02."
	case lab.ScenarioLockLoss:
		return "RTSP SETUP and PLAY still succeed, while /api/tuners reports frontend.state=lost with deterministic zero signal and high BER/PER for lock-loss UI and retry handling."
	case lab.ScenarioSignalRecovery:
		return "RTSP SETUP and PLAY still succeed, while /api/tuners reports frontend.state=recovering before returning to locked after the deterministic lock window."
	case lab.ScenarioSlowLock:
		return "RTSP SETUP and PLAY still succeed, while /api/tuners reports frontend.state=tuning with lock_ms=1200 for slow-lock UI and timeout tolerance tests."
	default:
		return ""
	}
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
		"Start implementation work on a codex/ branch; do not work directly on main.",
		"Poll /api/agent/context or /desc.xml before client tests.",
		"Use SATIP_TEST_HTTP_URL and SATIP_TEST_RTSP_URL instead of hard-coded ports.",
		"Call POST /api/reset between independent integration tests.",
		"Use POST /api/scenario to exercise failure handling, then restore normal.",
		"Assert M3U, XMLTV, EIT p/f, RTSP setup, PLAY, and RTP behavior separately.",
		"Build and smoke-test the container before PRs that change runtime behavior, Docker, CI, media generation, or advertised lab contracts.",
		"Open a PR with verification evidence and client-facing compatibility notes.",
		"Request or spawn a PR review pass before merge and address confirmed review issues.",
		"Re-run relevant tests after review fixes; rebuild and smoke-test the container again when the container path was required.",
		"Publish containers and merge only with explicit maintainer approval or through the release workflow.",
		"Update docs/agents and /api/agent/context whenever new lab capabilities, config, scenarios, or companion tools are added.",
	}
}
