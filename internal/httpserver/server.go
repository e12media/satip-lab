package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/e12media/satip-lab/internal/channels"
	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/epg"
	"github.com/e12media/satip-lab/internal/lab"
)

type Server struct {
	cfg    config.Config
	server *http.Server
	ln     net.Listener
	lab    *lab.Manager
	reset  func()
}

func New(cfg config.Config, labManager *lab.Manager, reset ...func()) *Server {
	if labManager == nil {
		labManager = lab.NewManager(lab.DefaultCatalog(), cfg.TunerCount)
	}
	var resetFunc func()
	if len(reset) > 0 {
		resetFunc = reset[0]
	}
	return &Server{cfg: cfg, lab: labManager, reset: resetFunc}
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", s.cfg.BindAddress, s.cfg.HTTPPort),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("satip-lab HTTP server stopped: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleStatus)
	mux.HandleFunc("/index.html", s.handleStatus)
	mux.HandleFunc("/desc.xml", s.handleDesc)
	if path := s.cfg.CompatibilityProfile().Device.DescriptionPath; path != "" && path != "/desc.xml" {
		mux.HandleFunc(path, s.handleDesc)
	}
	mux.HandleFunc("/channels.m3u", s.handleM3U)
	if path := s.cfg.CompatibilityProfile().Device.XSatipM3U; path != "" && path != "/channels.m3u" {
		mux.HandleFunc(path, s.handleM3U)
	}
	mux.HandleFunc("/epg/xmltv.xml", s.handleXMLTV)
	mux.HandleFunc("/api/agent/context", s.handleAPIAgentContext)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/catalog", s.handleAPICatalog)
	mux.HandleFunc("/api/muxes", s.handleAPIMuxes)
	mux.HandleFunc("/api/services", s.handleAPIServices)
	mux.HandleFunc("/api/tuners", s.handleAPITuners)
	mux.HandleFunc("/api/sessions", s.handleAPISessions)
	mux.HandleFunc("/api/events", s.handleAPIEvents)
	mux.HandleFunc("/api/clock", s.handleAPIClock)
	mux.HandleFunc("/api/config/schema", s.handleAPIConfigSchema)
	mux.HandleFunc("/api/schema", s.handleAPISchema)
	mux.HandleFunc("/api/scenario", s.handleAPIScenario)
	mux.HandleFunc("/api/reset", s.handleAPIReset)
	return mux
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>satip-lab</title></head>
<body>
<h1>satip-lab</h1>
<p>Local SAT&gt;IP lab server for client developers.</p>
<ul>
<li><a href="/desc.xml">Device description</a></li>
<li><a href="/channels.m3u">Channel list (M3U)</a></li>
</ul>
<p>Startup scenario: %s</p>
<p>Runtime scenario: %s</p>
<p>RTSP port: %d</p>
<p>Transport stream: %s</p>
<p>Compatibility profile: %s</p>
</body>
</html>`, s.cfg.ScenarioName(), s.lab.Scenario().Name, s.cfg.RTSPPort, s.cfg.TransportStreamPath, s.cfg.CompatibilityProfile().Name)
}

func (s *Server) handleDesc(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(channels.BuildDeviceDescriptionXML(s.cfg.PresentationURL(), s.cfg.TunerCount, s.cfg.CompatibilityProfile())))
}

func (s *Server) handleM3U(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	if s.lab.Scenario().Name == lab.ScenarioBadM3U {
		_, _ = w.Write([]byte("#EXTM3U\n# satip-lab:bad_m3u\n#EXTINF:-1,broken SAT>IP entry\nnot-a-satip-url\n"))
		return
	}
	_, _ = w.Write([]byte(channels.BuildM3U(s.cfg.PublicHost, s.cfg.EffectivePublicRTSPPort(), s.lab.Catalog().Channels())))
}

func (s *Server) handleXMLTV(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	clock, err := epg.ParseClock(s.cfg.EPGClock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, meta, err := epg.GenerateXMLTV(s.lab.Catalog(), clock, epg.Options{Scenario: s.lab.Scenario()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
	_, _ = w.Write(body)
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Status())
}

func (s *Server) handleAPIAgentContext(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, buildAgentContext(s.cfg, s.lab))
}

func (s *Server) handleAPICatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Catalog())
}

func (s *Server) handleAPIMuxes(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Catalog().Muxes)
}

func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Catalog().Services)
}

func (s *Server) handleAPITuners(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Status().Tuners)
}

func (s *Server) handleAPISessions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Status().Sessions)
}

func (s *Server) handleAPIEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, s.lab.Status().Events)
}

func (s *Server) handleAPIClock(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	clock, err := epg.ParseClock(s.cfg.EPGClock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, clock)
}

func (s *Server) handleAPIConfigSchema(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, config.Schema())
}

func (s *Server) handleAPISchema(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, Schema())
}

func (s *Server) handleAPIScenario(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.lab.Scenario())
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			ServiceID   string `json:"service_id"`
			MuxID       string `json:"mux_id"`
			DurationMin int    `json:"duration_min"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.lab.SetScenarioOptions(req.Name, req.ServiceID, req.MuxID, req.DurationMin); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, s.lab.Scenario())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAPIReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.reset != nil {
		s.reset()
	}
	s.lab.Reset()
	writeJSON(w, s.lab.Status())
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}
