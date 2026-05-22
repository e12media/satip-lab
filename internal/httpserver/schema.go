package httpserver

const APISchemaVersion = "1.3"

type APISchema struct {
	Version   string              `json:"version"`
	Endpoints []APISchemaEndpoint `json:"endpoints"`
	Models    []APISchemaModel    `json:"models"`
}

type APISchemaEndpoint struct {
	Path        string   `json:"path"`
	Methods     []string `json:"methods"`
	Description string   `json:"description"`
}

type APISchemaModel struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

func Schema() APISchema {
	return APISchema{
		Version: APISchemaVersion,
		Endpoints: []APISchemaEndpoint{
			{Path: "/api/agent/context", Methods: []string{"GET"}, Description: "Coding agent bootstrap context with URLs, scenarios, docs, and recommended checks."},
			{Path: "/api/config/schema", Methods: []string{"GET"}, Description: "Versioned configuration contract."},
			{Path: "/api/clock", Methods: []string{"GET"}, Description: "Current deterministic lab clock for EPG generation."},
			{Path: "/api/schema", Methods: []string{"GET"}, Description: "Versioned lab API contract."},
			{Path: "/api/status", Methods: []string{"GET"}, Description: "Full lab status."},
			{Path: "/api/catalog", Methods: []string{"GET"}, Description: "Mux and service catalog."},
			{Path: "/api/muxes", Methods: []string{"GET"}, Description: "Mux catalog entries."},
			{Path: "/api/services", Methods: []string{"GET"}, Description: "Service catalog entries."},
			{Path: "/api/tuners", Methods: []string{"GET"}, Description: "Simulated tuner state."},
			{Path: "/api/sessions", Methods: []string{"GET"}, Description: "Active RTSP lab sessions."},
			{Path: "/api/events", Methods: []string{"GET"}, Description: "Recent lab events."},
			{Path: "/api/scenario", Methods: []string{"GET", "POST"}, Description: "Runtime scenario state and switching."},
			{Path: "/api/reset", Methods: []string{"POST"}, Description: "Reset lab sessions and tuner state."},
			{Path: "/epg/xmltv.xml", Methods: []string{"GET"}, Description: "Deterministic XMLTV EPG for the lab catalog."},
		},
		Models: []APISchemaModel{
			{Name: "agent_context", Fields: []string{"version", "urls", "test_env", "catalog", "features", "runtime", "compatibility", "scenarios", "docs", "recommended_checks"}},
			{Name: "clock", Fields: []string{"mode", "now", "tz"}},
			{Name: "catalog", Fields: []string{"muxes", "services"}},
			{Name: "status", Fields: []string{"tuners", "sessions", "events"}},
			{Name: "tuner", Fields: []string{"id", "state", "mux_id", "sessions"}},
			{Name: "session", Fields: []string{"id", "state", "tuner_id", "service_id", "service", "mux_id", "pids", "pids_all", "client", "created_at", "updated_at"}},
			{Name: "event", Fields: []string{"at", "type", "session_id", "tuner_id", "service_id", "mux_id", "message"}},
			{Name: "scenario", Fields: []string{"name", "description", "service_id", "mux_id", "duration_min"}},
			{Name: "mux", Fields: []string{"id", "src", "freq", "pol", "sr", "msys"}},
			{Name: "service", Fields: []string{"id", "number", "name", "group", "tvg_id", "mux_id", "service_id", "pmt_pid", "video_pid", "audio_pid"}},
		},
	}
}
