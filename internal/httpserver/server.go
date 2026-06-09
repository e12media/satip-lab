package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"

	"github.com/e12media/satip-lab/internal/channels"
	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/epg"
	"github.com/e12media/satip-lab/internal/lab"
)

type Server struct {
	cfg       config.Config
	server    *http.Server
	ln        net.Listener
	lab       *lab.Manager
	reset     func()
	startedAt time.Time
}

type Status struct {
	Tuners   []lab.Tuner    `json:"tuners"`
	Sessions []lab.Session  `json:"sessions"`
	Events   []lab.Event    `json:"events"`
	Hardware HardwareStatus `json:"hardware"`
}

type HardwareStatus struct {
	LabOnly   bool             `json:"lab_only"`
	StartedAt time.Time        `json:"started_at"`
	UptimeMS  int64            `json:"uptime_ms"`
	Identity  HardwareIdentity `json:"identity"`
	Streams   HardwareStreams  `json:"streams"`
	Tuners    HardwareTuners   `json:"tuners"`
	Network   HardwareNetwork  `json:"network"`
}

type HardwareIdentity struct {
	FriendlyName string `json:"friendly_name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	ModelNumber  string `json:"model_number,omitempty"`
	Profile      string `json:"profile"`
	Firmware     string `json:"firmware"`
}

type HardwareStreams struct {
	Active  int `json:"active"`
	Playing int `json:"playing"`
	Setup   int `json:"setup"`
	Paused  int `json:"paused"`
}

type HardwareTuners struct {
	Total int `json:"total"`
	InUse int `json:"in_use"`
	Idle  int `json:"idle"`
}

type HardwareNetwork struct {
	HTTPPort      int `json:"http_port"`
	RTSPPort      int `json:"rtsp_port"`
	SSDPPort      int `json:"ssdp_port"`
	RTSPSessions  int `json:"rtsp_sessions"`
	RTPStreams    int `json:"rtp_streams"`
	FrontendLocks int `json:"frontend_locks"`
	RecentEvents  int `json:"recent_events"`
}

func New(cfg config.Config, labManager *lab.Manager, reset ...func()) *Server {
	if labManager == nil {
		labManager = lab.NewManager(lab.DefaultCatalog(), cfg.TunerCount)
	}
	var resetFunc func()
	if len(reset) > 0 {
		resetFunc = reset[0]
	}
	return &Server{cfg: cfg, lab: labManager, reset: resetFunc, startedAt: time.Now().UTC()}
}

func (s *Server) status() Status {
	labStatus := s.lab.Status()
	return Status{
		Tuners:   labStatus.Tuners,
		Sessions: labStatus.Sessions,
		Events:   labStatus.Events,
		Hardware: s.hardwareStatus(labStatus),
	}
}

func (s *Server) hardwareStatus(status lab.Status) HardwareStatus {
	profile := s.cfg.CompatibilityProfile()
	streams := HardwareStreams{Active: len(status.Sessions)}
	for _, session := range status.Sessions {
		switch session.State {
		case "playing":
			streams.Playing++
		case "paused":
			streams.Paused++
		case "setup":
			streams.Setup++
		}
	}
	tuners := HardwareTuners{Total: len(status.Tuners)}
	frontendLocks := 0
	for _, tuner := range status.Tuners {
		if tuner.State == "idle" {
			tuners.Idle++
		} else {
			tuners.InUse++
		}
		switch tuner.Frontend.State {
		case lab.FrontendLocked, lab.FrontendDegraded:
			frontendLocks++
		}
	}
	uptime := time.Since(s.startedAt)
	if uptime < 0 {
		uptime = 0
	}
	return HardwareStatus{
		LabOnly:   true,
		StartedAt: s.startedAt,
		UptimeMS:  uptime.Milliseconds(),
		Identity: HardwareIdentity{
			FriendlyName: profile.Device.FriendlyName,
			Manufacturer: profile.Device.Manufacturer,
			Model:        profile.Device.ModelName,
			ModelNumber:  profile.Device.ModelNumber,
			Profile:      profile.Name,
			Firmware:     "satip-lab simulated",
		},
		Streams: streams,
		Tuners:  tuners,
		Network: HardwareNetwork{
			HTTPPort:      s.cfg.EffectivePublicHTTPPort(),
			RTSPPort:      s.cfg.EffectivePublicRTSPPort(),
			SSDPPort:      s.cfg.SSDPort,
			RTSPSessions:  len(status.Sessions),
			RTPStreams:    streams.Playing,
			FrontendLocks: frontendLocks,
			RecentEvents:  len(status.Events),
		},
	}
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
	status := s.status()
	frontendState := "idle"
	if len(status.Tuners) > 0 {
		frontendState = status.Tuners[0].Frontend.State
	}
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
<h2>Hardware status</h2>
<p>Device: %s (%s)</p>
<p>Firmware: %s</p>
<p>Uptime: %d ms</p>
<p>Active streams: %d</p>
<p>Tuners in use: %d / %d</p>
<p>Frontend state: %s</p>
<p><a href="/api/status">JSON status</a></p>
</body>
</html>`,
		s.cfg.ScenarioName(),
		s.lab.Scenario().Name,
		s.cfg.RTSPPort,
		html.EscapeString(s.cfg.TransportStreamPath),
		html.EscapeString(s.cfg.CompatibilityProfile().Name),
		html.EscapeString(status.Hardware.Identity.FriendlyName),
		html.EscapeString(status.Hardware.Identity.Model),
		html.EscapeString(status.Hardware.Identity.Firmware),
		status.Hardware.UptimeMS,
		status.Hardware.Streams.Active,
		status.Hardware.Tuners.InUse,
		status.Hardware.Tuners.Total,
		html.EscapeString(frontendState),
	)
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
	writeJSON(w, s.status())
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
			Name        string                     `json:"name"`
			ServiceID   string                     `json:"service_id"`
			MuxID       string                     `json:"mux_id"`
			DurationMin int                        `json:"duration_min"`
			Timeline    []lab.ScenarioTimelineStep `json:"timeline"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		var err error
		if len(req.Timeline) > 0 {
			err = s.lab.SetScenarioTimeline(req.Timeline)
		} else {
			err = s.lab.SetScenarioOptions(req.Name, req.ServiceID, req.MuxID, req.DurationMin)
		}
		if err != nil {
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
