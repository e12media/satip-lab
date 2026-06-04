package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/httpserver"
	"github.com/e12media/satip-lab/internal/lab"
)

func TestAPICatalogMuxesAndServices(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{}, labManager)
	handler := server.Handler()

	for _, tc := range []struct {
		path    string
		minSize int
	}{
		{path: "/api/catalog", minSize: 1},
		{path: "/api/muxes", minSize: 4},
		{path: "/api/services", minSize: 5},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status: got %d body=%s", tc.path, rec.Code, rec.Body.String())
		}
		var decoded any
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s invalid json: %v", tc.path, err)
		}
		if tc.path != "/api/catalog" {
			items, ok := decoded.([]any)
			if !ok || len(items) < tc.minSize {
				t.Fatalf("%s expected at least %d items, got %#v", tc.path, tc.minSize, decoded)
			}
		}
	}
}

func TestReadOnlyAPIEndpointsRejectNonGET(t *testing.T) {
	handler := httpserver.New(config.Config{}, lab.NewManager(lab.DefaultCatalog(), 1)).Handler()

	for _, path := range []string{
		"/api/agent/context",
		"/api/config/schema",
		"/api/clock",
		"/api/schema",
		"/api/status",
		"/api/catalog",
		"/api/muxes",
		"/api/services",
		"/api/tuners",
		"/api/sessions",
		"/api/events",
		"/epg/xmltv.xml",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status: got %d", path, rec.Code)
		}
	}
}

func TestAPIAgentContextReturnsCodingAgentBootstrap(t *testing.T) {
	cfg := config.Config{
		PublicHost:     "satip.test",
		HTTPPort:       8875,
		RTSPPort:       554,
		PublicHTTPPort: 18875,
		PublicRTSPPort: 1554,
		TunerCount:     2,
		EPGClock:       "fixed:2026-03-29T01:30:00+01:00",
	}
	handler := httpserver.New(cfg, lab.NewManager(lab.DefaultCatalog(), 2)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/agent/context", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agent/context status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Version string `json:"version"`
		URLs    struct {
			HTTPBaseURL string `json:"http_base_url"`
			RTSPBaseURL string `json:"rtsp_base_url"`
			M3U         string `json:"m3u"`
			XMLTV       string `json:"xmltv"`
			Clock       string `json:"clock"`
			Schema      string `json:"schema"`
		} `json:"urls"`
		TestEnv map[string]string `json:"test_env"`
		Catalog struct {
			ServiceCount int    `json:"service_count"`
			MuxCount     int    `json:"mux_count"`
			Source       string `json:"source"`
			CatalogPath  string `json:"catalog_path"`
			FixturePath  string `json:"fixture_path"`
			SampleRTSP   string `json:"sample_rtsp_url"`
		} `json:"catalog"`
		Features map[string]bool `json:"features"`
		Runtime  struct {
			Tuners        int    `json:"tuners"`
			Scenario      string `json:"scenario"`
			Profile       string `json:"profile"`
			ReadinessPath string `json:"readiness_path"`
		} `json:"runtime"`
		Compatibility struct {
			ActiveProfile     string   `json:"active_profile"`
			AvailableProfiles []string `json:"available_profiles"`
			CorpusPath        string   `json:"corpus_path"`
		} `json:"compatibility"`
		Scenarios []struct {
			Name                  string `json:"name"`
			SupportsTarget        bool   `json:"supports_target"`
			ClientExpectationHint string `json:"client_expectation_hint"`
		} `json:"scenarios"`
		Docs []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"docs"`
		RecommendedChecks []string `json:"recommended_checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != "1.0" {
		t.Fatalf("version: got %q", got.Version)
	}
	if got.URLs.HTTPBaseURL != "http://satip.test:18875" || got.URLs.RTSPBaseURL != "rtsp://satip.test:1554/" {
		t.Fatalf("urls: %+v", got.URLs)
	}
	if got.URLs.XMLTV != "http://satip.test:18875/epg/xmltv.xml" || got.URLs.Clock != "http://satip.test:18875/api/clock" {
		t.Fatalf("epg urls: %+v", got.URLs)
	}
	if got.TestEnv["SATIP_TEST_HTTP_URL"] != "http://satip.test:18875" || got.TestEnv["SATIP_TEST_RTSP_URL"] != "rtsp://satip.test:1554/" {
		t.Fatalf("test env: %+v", got.TestEnv)
	}
	if got.Catalog.ServiceCount != 5 || got.Catalog.MuxCount < 4 || !strings.Contains(got.Catalog.SampleRTSP, "rtsp://satip.test:1554/") {
		t.Fatalf("catalog: %+v", got.Catalog)
	}
	if got.Catalog.Source != "built_in" || got.Catalog.CatalogPath != "" || got.Catalog.FixturePath != "fixtures/astra-19.2e-dach.yaml" {
		t.Fatalf("catalog source: %+v", got.Catalog)
	}
	for _, feature := range []string{"custom_catalogs", "compatibility_profiles", "xmltv_epg", "eit_present_following", "rtsp_rtp_smoke", "runtime_scenarios"} {
		if !got.Features[feature] {
			t.Fatalf("missing feature %q in %+v", feature, got.Features)
		}
	}
	if got.Runtime.Tuners != 2 || got.Runtime.Scenario != lab.ScenarioNormal || got.Runtime.Profile != "generic-satip-1.2" || got.Runtime.ReadinessPath != "/api/agent/context" {
		t.Fatalf("runtime: %+v", got.Runtime)
	}
	if got.Compatibility.ActiveProfile != "generic-satip-1.2" || len(got.Compatibility.AvailableProfiles) < 3 || got.Compatibility.CorpusPath != "docs/compatibility/servers.md" {
		t.Fatalf("compatibility: %+v", got.Compatibility)
	}
	if len(got.Scenarios) < 10 || got.Scenarios[0].Name != lab.ScenarioNormal {
		t.Fatalf("scenarios: %+v", got.Scenarios)
	}
	for _, scenario := range got.Scenarios {
		if scenario.Name == lab.ScenarioRTPStop && !strings.Contains(scenario.ClientExpectationHint, "3 RTP packets") {
			t.Fatalf("rtp_stop expectation hint: %+v", scenario)
		}
		if scenario.Name == lab.ScenarioRTPLoss && !strings.Contains(scenario.ClientExpectationHint, "every third") {
			t.Fatalf("rtp_loss expectation hint: %+v", scenario)
		}
	}
	if len(got.Docs) == 0 || got.Docs[0].Path != "docs/agents/README.md" {
		t.Fatalf("docs: %+v", got.Docs)
	}
	for _, hint := range []string{"codex/", "container", "Open a PR", "PR review", "Re-run relevant tests", "Publish containers and merge only"} {
		if !containsStringWith(got.RecommendedChecks, hint) {
			t.Fatalf("recommended checks should include %q workflow hint: %+v", hint, got.RecommendedChecks)
		}
	}
}

func TestAPIClockReturnsDeterministicLabClock(t *testing.T) {
	handler := httpserver.New(config.Config{EPGClock: "fixed:2026-03-29T01:30:00+01:00"}, lab.NewManager(lab.DefaultCatalog(), 1)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/clock", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/clock status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Mode string `json:"mode"`
		Now  string `json:"now"`
		TZ   string `json:"tz"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "fixed" || got.Now != "2026-03-29T01:30:00+01:00" || got.TZ != "Europe/Berlin" {
		t.Fatalf("clock: %+v", got)
	}
}

func TestAPIScenarioCanBeReadAndChanged(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{}, labManager)
	handler := server.Handler()

	getReq := httptest.NewRequest(http.MethodGet, "/api/scenario", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/scenario status: got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var initial struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Name != lab.ScenarioNormal {
		t.Fatalf("initial scenario: got %q", initial.Name)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/scenario", bytes.NewBufferString(`{"name":"no_signal","service_id":"zdf-hd"}`))
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /api/scenario status: got %d body=%s", postRec.Code, postRec.Body.String())
	}
	var updated struct {
		Name      string `json:"name"`
		ServiceID string `json:"service_id"`
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != lab.ScenarioNoSignal {
		t.Fatalf("updated scenario: got %q", updated.Name)
	}
	if updated.ServiceID != "zdf-hd" {
		t.Fatalf("updated scenario service target: got %q", updated.ServiceID)
	}
}

func TestAPIScenarioAcceptsEPGGapDuration(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{}, labManager)
	handler := server.Handler()

	postReq := httptest.NewRequest(http.MethodPost, "/api/scenario", bytes.NewBufferString(`{"name":"epg_gap","service_id":"arte-hd","duration_min":90}`))
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /api/scenario status: got %d body=%s", postRec.Code, postRec.Body.String())
	}
	var updated struct {
		Name        string `json:"name"`
		ServiceID   string `json:"service_id"`
		DurationMin int    `json:"duration_min"`
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != lab.ScenarioEPGGap || updated.ServiceID != "arte-hd" || updated.DurationMin != 90 {
		t.Fatalf("updated scenario: %+v", updated)
	}
}

func TestAPIScenarioRejectsUnknownName(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{}, labManager)
	handler := server.Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/scenario", bytes.NewBufferString(`{"name":"bad_moon"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := labManager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("scenario should remain normal after rejection, got %q", got)
	}
}

func TestAPIConfigSchema(t *testing.T) {
	handler := httpserver.New(config.Config{}, lab.NewManager(lab.DefaultCatalog(), 1)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/config/schema", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config/schema status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Version   string `json:"version"`
		Variables []struct {
			Name string `json:"name"`
		} `json:"variables"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != config.SchemaVersion || len(got.Variables) == 0 || got.Variables[0].Name != "SATIP_LAB_BIND" {
		t.Fatalf("unexpected config schema: %+v", got)
	}
}

func TestAPISchema(t *testing.T) {
	handler := httpserver.New(config.Config{}, lab.NewManager(lab.DefaultCatalog(), 1)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/schema", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/schema status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Version   string `json:"version"`
		Endpoints []struct {
			Path    string   `json:"path"`
			Methods []string `json:"methods"`
		} `json:"endpoints"`
		Models []struct {
			Name   string   `json:"name"`
			Fields []string `json:"fields"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != httpserver.APISchemaVersion {
		t.Fatalf("schema version: got %q", got.Version)
	}
	wantEndpoints := map[string][]string{
		"/api/agent/context": {"GET"},
		"/api/config/schema": {"GET"},
		"/api/clock":         {"GET"},
		"/api/schema":        {"GET"},
		"/api/status":        {"GET"},
		"/api/catalog":       {"GET"},
		"/api/muxes":         {"GET"},
		"/api/services":      {"GET"},
		"/api/tuners":        {"GET"},
		"/api/sessions":      {"GET"},
		"/api/events":        {"GET"},
		"/api/scenario":      {"GET", "POST"},
		"/api/reset":         {"POST"},
		"/epg/xmltv.xml":     {"GET"},
	}
	if len(got.Endpoints) != len(wantEndpoints) {
		t.Fatalf("endpoint count: got %d want %d", len(got.Endpoints), len(wantEndpoints))
	}
	for _, endpoint := range got.Endpoints {
		wantMethods, ok := wantEndpoints[endpoint.Path]
		if !ok {
			t.Fatalf("unexpected endpoint: %+v", endpoint)
		}
		if !sameStrings(endpoint.Methods, wantMethods) {
			t.Fatalf("%s methods: got %#v want %#v", endpoint.Path, endpoint.Methods, wantMethods)
		}
	}
	wantModels := map[string][]string{
		"agent_context": {"version", "urls", "test_env", "catalog", "features", "runtime", "compatibility", "scenarios", "docs", "recommended_checks"},
		"catalog":       {"muxes", "services"},
		"status":        {"tuners", "sessions", "events"},
		"tuner":         {"id", "state", "mux_id", "sessions"},
		"session":       {"id", "state", "tuner_id", "service_id", "service", "mux_id", "pids", "pids_all", "client", "created_at", "updated_at"},
		"event":         {"at", "type", "session_id", "tuner_id", "service_id", "mux_id", "message"},
		"clock":         {"mode", "now", "tz"},
		"scenario":      {"name", "description", "service_id", "mux_id", "duration_min"},
		"mux":           {"id", "src", "freq", "pol", "sr", "msys"},
		"service":       {"id", "number", "name", "group", "tvg_id", "mux_id", "service_id", "pmt_pid", "video_pid", "audio_pid"},
	}
	if len(got.Models) != len(wantModels) {
		t.Fatalf("model count: got %d want %d", len(got.Models), len(wantModels))
	}
	for _, model := range got.Models {
		wantFields, ok := wantModels[model.Name]
		if !ok {
			t.Fatalf("unexpected model: %+v", model)
		}
		if !sameStrings(model.Fields, wantFields) {
			t.Fatalf("%s fields: got %#v want %#v", model.Name, model.Fields, wantFields)
		}
	}
}

func TestXMLTVEndpointReturnsDeterministicEPG(t *testing.T) {
	handler := httpserver.New(config.Config{EPGClock: "fixed:2026-03-29T01:30:00+01:00"}, lab.NewManager(lab.DefaultCatalog(), 1)).Handler()

	req := httptest.NewRequest(http.MethodGet, "/epg/xmltv.xml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /epg/xmltv.xml status: got %d body=%s", rec.Code, body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/xml; charset=utf-8" {
		t.Fatalf("content type: got %q", got)
	}
	if got := rec.Header().Get("Last-Modified"); got == "" {
		t.Fatal("missing Last-Modified header")
	}
	for _, want := range []string{
		`<channel id="daserste.de">`,
		`<display-name>Das Erste HD</display-name>`,
		`start="20260329013000 +0100"`,
		`channel="zdf.de"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in XMLTV body:\n%s", want, body)
		}
	}
}

func TestXMLTVEndpointAppliesEPGStaleLastModified(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := labManager.SetScenario(lab.ScenarioEPGStale); err != nil {
		t.Fatal(err)
	}
	handler := httpserver.New(config.Config{EPGClock: "fixed:2026-03-29T01:30:00+01:00"}, labManager).Handler()

	req := httptest.NewRequest(http.MethodGet, "/epg/xmltv.xml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /epg/xmltv.xml status: got %d body=%s", rec.Code, rec.Body.String())
	}
	want := "Fri, 27 Mar 2026 00:30:00 GMT"
	if got := rec.Header().Get("Last-Modified"); got != want {
		t.Fatalf("Last-Modified: got %q want %q", got, want)
	}
}

func TestBadM3UScenarioReturnsMalformedChannelList(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	if err := labManager.SetScenario(lab.ScenarioBadM3U); err != nil {
		t.Fatal(err)
	}
	server := httpserver.New(config.Config{PublicHost: "127.0.0.1", RTSPPort: 554}, labManager)
	req := httptest.NewRequest(http.MethodGet, "/channels.m3u", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, body)
	}
	if !strings.Contains(body, "satip-lab:bad_m3u") {
		t.Fatalf("missing bad_m3u marker: %q", body)
	}
	if strings.Contains(body, "rtsp://") {
		t.Fatalf("bad_m3u should not expose usable RTSP URLs: %q", body)
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

func containsStringWith(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
