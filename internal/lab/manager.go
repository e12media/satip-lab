package lab

import (
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidTune      = errors.New("invalid tune")
	ErrServiceNotFound  = errors.New("service not found")
	ErrNoTunerAvailable = errors.New("no tuner available")
	ErrSessionNotFound  = errors.New("session not found")
	ErrUnknownScenario  = errors.New("unknown scenario")
	ErrScenarioTarget   = errors.New("scenario does not support service or mux targets")
	ErrScenarioDuration = errors.New("scenario does not support duration_min")
	ErrNoSignal         = errors.New("no signal")
)

type Manager struct {
	mu       sync.Mutex
	catalog  Catalog
	tuners   []Tuner
	sessions map[string]Session
	events   []Event
	scenario Scenario
}

type SetupResult struct {
	Session Session
	Service Service
	Mux     Mux
	TunerID int
}

type Session struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	TunerID   int       `json:"tuner_id"`
	ServiceID string    `json:"service_id"`
	Service   string    `json:"service"`
	MuxID     string    `json:"mux_id"`
	PIDs      []int     `json:"pids,omitempty"`
	PIDsAll   bool      `json:"pids_all,omitempty"`
	Client    string    `json:"client"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Tuner struct {
	ID       int           `json:"id"`
	State    string        `json:"state"`
	MuxID    string        `json:"mux_id,omitempty"`
	Sessions []string      `json:"sessions,omitempty"`
	Frontend TunerFrontend `json:"frontend"`
}

type TunerFrontend struct {
	State          string     `json:"state"`
	SignalStrength int        `json:"signal_strength"`
	SNRDB          float64    `json:"snr_db"`
	BER            float64    `json:"ber"`
	PER            float64    `json:"per"`
	LockMS         int        `json:"lock_ms"`
	LastLockChange *time.Time `json:"last_lock_change,omitempty"`
}

type Event struct {
	At        time.Time `json:"at"`
	Type      string    `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	TunerID   int       `json:"tuner_id,omitempty"`
	ServiceID string    `json:"service_id,omitempty"`
	MuxID     string    `json:"mux_id,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type Status struct {
	Tuners   []Tuner   `json:"tuners"`
	Sessions []Session `json:"sessions"`
	Events   []Event   `json:"events"`
}

const (
	ScenarioNormal           = "normal"
	ScenarioNoSignal         = "no_signal"
	ScenarioBadM3U           = "bad_m3u"
	ScenarioTunerBusy        = "tuner_busy"
	ScenarioRTPStop          = "rtp_stop"
	ScenarioSlowRTSP         = "slow_rtsp"
	ScenarioMalformedPSI     = "malformed_psi"
	ScenarioRTPLoss          = "rtp_loss"
	ScenarioRTPJitter        = "rtp_jitter"
	ScenarioContinuityErrors = "cc_errors"
	ScenarioEPGGap           = "epg_gap"
	ScenarioEPGMismatch      = "epg_mismatch"
	ScenarioEPGStale         = "epg_stale"
	ScenarioSignalDegraded   = "signal_degraded"
	ScenarioLockLoss         = "lock_loss"
	ScenarioSlowLock         = "slow_lock"
)

const (
	FrontendIdle     = "idle"
	FrontendTuning   = "tuning"
	FrontendLocked   = "locked"
	FrontendDegraded = "degraded"
	FrontendLost     = "lost"
)

type Scenario struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ServiceID   string `json:"service_id,omitempty"`
	MuxID       string `json:"mux_id,omitempty"`
	DurationMin int    `json:"duration_min,omitempty"`
}

func NewManager(catalog Catalog, tunerCount int) *Manager {
	if tunerCount <= 0 {
		tunerCount = 1
	}
	tuners := make([]Tuner, tunerCount)
	for i := range tuners {
		tuners[i] = Tuner{ID: i + 1, State: "idle", Frontend: idleFrontend()}
	}
	return &Manager{
		catalog:  catalog,
		tuners:   tuners,
		sessions: make(map[string]Session),
		scenario: scenarioByName(ScenarioNormal),
	}
}

func (m *Manager) Catalog() Catalog {
	return m.catalog
}

func (m *Manager) Setup(sessionID, rawQuery, client string) (SetupResult, error) {
	service, mux, err := m.catalog.MatchService(rawQuery)
	if err != nil {
		m.record(Event{Type: "setup_rejected", Message: err.Error()})
		return SetupResult{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scenario.Name == ScenarioNoSignal && m.scenario.AppliesTo(service, mux) {
		m.recordLocked(Event{Type: "setup_rejected", ServiceID: service.ID, MuxID: mux.ID, Message: ErrNoSignal.Error()})
		return SetupResult{}, ErrNoSignal
	}
	if m.scenario.Name == ScenarioTunerBusy {
		m.recordLocked(Event{Type: "tuner_busy", ServiceID: service.ID, MuxID: mux.ID, Message: ErrNoTunerAvailable.Error()})
		return SetupResult{}, ErrNoTunerAvailable
	}

	pids, pidsAll, err := requestedPIDs(rawQuery, service)
	if err != nil {
		m.recordLocked(Event{Type: "setup_rejected", ServiceID: service.ID, MuxID: mux.ID, Message: ErrInvalidTune.Error()})
		return SetupResult{}, ErrInvalidTune
	}

	tunerID, err := m.allocateTunerLocked(mux.ID)
	if err != nil {
		m.recordLocked(Event{Type: "tuner_busy", ServiceID: service.ID, MuxID: mux.ID, Message: err.Error()})
		return SetupResult{}, err
	}

	now := time.Now().UTC()
	session := Session{
		ID:        sessionID,
		State:     "setup",
		TunerID:   tunerID,
		ServiceID: service.ID,
		Service:   service.Name,
		MuxID:     mux.ID,
		PIDs:      pids,
		PIDsAll:   pidsAll,
		Client:    client,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.sessions[sessionID] = session
	m.addSessionToTunerLocked(tunerID, sessionID)
	m.recomputeTunerFrontendLocked(tunerID, now)
	m.recordLocked(Event{Type: "session_setup", SessionID: sessionID, TunerID: tunerID, ServiceID: service.ID, MuxID: mux.ID})

	return SetupResult{Session: session, Service: service, Mux: mux, TunerID: tunerID}, nil
}

func (m *Manager) Play(sessionID string) (SetupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return SetupResult{}, ErrSessionNotFound
	}
	service, _ := m.catalog.ServiceByID(session.ServiceID)
	mux, _ := m.catalog.MuxByID(session.MuxID)
	session.State = "playing"
	session.UpdatedAt = time.Now().UTC()
	m.sessions[sessionID] = session
	m.recordLocked(Event{Type: "play_started", SessionID: sessionID, TunerID: session.TunerID, ServiceID: service.ID, MuxID: mux.ID})
	return SetupResult{Session: session, Service: service, Mux: mux, TunerID: session.TunerID}, nil
}

func (m *Manager) Pause(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	session.State = "paused"
	session.UpdatedAt = time.Now().UTC()
	m.sessions[sessionID] = session
	m.recordLocked(Event{Type: "play_paused", SessionID: sessionID, TunerID: session.TunerID, ServiceID: session.ServiceID, MuxID: session.MuxID})
	return nil
}

func (m *Manager) UpdatePIDs(sessionID, rawQuery string) error {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ErrInvalidTune
	}
	if _, ok := values["pids"]; !ok {
		if _, ok := values["addpids"]; !ok {
			if _, ok := values["delpids"]; !ok {
				return nil
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	next := append([]int(nil), session.PIDs...)
	pidsAll := session.PIDsAll
	if rawPIDs := values.Get("pids"); rawPIDs != "" {
		var err error
		next, pidsAll, err = parsePIDList(rawPIDs)
		if err != nil {
			return ErrInvalidTune
		}
	}
	if rawAdd := values.Get("addpids"); rawAdd != "" {
		added, addedAll, err := parsePIDList(rawAdd)
		if err != nil {
			return ErrInvalidTune
		}
		if addedAll {
			next = nil
			pidsAll = true
		} else if !pidsAll {
			for _, pid := range added {
				next = addPID(next, pid)
			}
		}
	}
	if rawDel := values.Get("delpids"); rawDel != "" {
		deleted, deletedAll, err := parsePIDList(rawDel)
		if err != nil {
			return ErrInvalidTune
		}
		if deletedAll {
			next = nil
			pidsAll = false
		} else if !pidsAll {
			for _, pid := range deleted {
				next = removePID(next, pid)
			}
		} else {
			return ErrInvalidTune
		}
	}
	sort.Ints(next)
	session.PIDs = next
	session.PIDsAll = pidsAll
	session.UpdatedAt = time.Now().UTC()
	m.sessions[sessionID] = session
	m.recordLocked(Event{Type: "pids_updated", SessionID: sessionID, TunerID: session.TunerID, ServiceID: session.ServiceID, MuxID: session.MuxID})
	return nil
}

func (m *Manager) Touch(sessionID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	session.UpdatedAt = now.UTC()
	m.sessions[sessionID] = session
	return nil
}

func (m *Manager) ExpireSessions(now time.Time, timeout time.Duration) []string {
	if timeout <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var expired []string
	for sessionID, session := range m.sessions {
		if now.Sub(session.UpdatedAt) <= timeout {
			continue
		}
		expired = append(expired, sessionID)
		delete(m.sessions, sessionID)
		m.removeSessionFromTunerLocked(session.TunerID, sessionID)
		m.recordLocked(Event{Type: "session_timeout", SessionID: sessionID, TunerID: session.TunerID, ServiceID: session.ServiceID, MuxID: session.MuxID})
	}
	sort.Strings(expired)
	return expired
}

func (m *Manager) Teardown(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	delete(m.sessions, sessionID)
	m.removeSessionFromTunerLocked(session.TunerID, sessionID)
	m.recordLocked(Event{Type: "session_closed", SessionID: sessionID, TunerID: session.TunerID, ServiceID: session.ServiceID, MuxID: session.MuxID})
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := make([]Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })

	tuners := make([]Tuner, len(m.tuners))
	copy(tuners, m.tuners)
	for i := range tuners {
		tuners[i].Sessions = append([]string(nil), tuners[i].Sessions...)
	}
	events := make([]Event, len(m.events))
	copy(events, m.events)
	return Status{Tuners: tuners, Sessions: sessions, Events: events}
}

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]Session)
	for i := range m.tuners {
		m.tuners[i].State = "idle"
		m.tuners[i].MuxID = ""
		m.tuners[i].Sessions = nil
		m.tuners[i].Frontend = idleFrontend()
	}
	m.recordLocked(Event{Type: "reset"})
}

func (m *Manager) Scenario() Scenario {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scenario
}

func requestedPIDs(rawQuery string, service Service) ([]int, bool, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, false, ErrInvalidTune
	}
	raw := values.Get("pids")
	if raw == "" {
		return defaultPIDs(service), false, nil
	}
	return parsePIDList(raw)
}

func defaultPIDs(service Service) []int {
	return []int{0, 17, service.PMTPID, service.VideoPID, service.AudioPID}
}

func parsePIDList(raw string) ([]int, bool, error) {
	if raw == "" {
		return nil, false, nil
	}
	if strings.EqualFold(raw, "all") {
		return nil, true, nil
	}
	seen := make(map[int]struct{})
	var pids []int
	for _, part := range strings.Split(raw, ",") {
		pid, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || pid < 0 || pid > 8191 {
			return nil, false, ErrInvalidTune
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids, false, nil
}

func addPID(pids []int, pid int) []int {
	for _, existing := range pids {
		if existing == pid {
			return pids
		}
	}
	return append(pids, pid)
}

func removePID(pids []int, pid int) []int {
	next := pids[:0]
	for _, existing := range pids {
		if existing != pid {
			next = append(next, existing)
		}
	}
	return next
}

func (m *Manager) SetScenario(name string) error {
	return m.SetScenarioTarget(name, "", "")
}

func (m *Manager) SetScenarioTarget(name, serviceID, muxID string) error {
	return m.SetScenarioOptions(name, serviceID, muxID, 0)
}

func (m *Manager) SetScenarioOptions(name, serviceID, muxID string, durationMin int) error {
	scenario, ok := lookupScenario(name)
	if !ok {
		return ErrUnknownScenario
	}
	if (serviceID != "" || muxID != "") && !scenario.SupportsTarget() {
		return ErrScenarioTarget
	}
	if durationMin > 0 && scenario.Name != ScenarioEPGGap {
		return ErrScenarioDuration
	}
	if serviceID != "" {
		if _, ok := m.catalog.ServiceByID(serviceID); !ok {
			return ErrServiceNotFound
		}
	}
	if muxID != "" {
		if _, ok := m.catalog.MuxByID(muxID); !ok {
			return ErrInvalidTune
		}
	}
	scenario.ServiceID = serviceID
	scenario.MuxID = muxID
	scenario.DurationMin = durationMin

	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenario = scenario
	m.recomputeAllFrontendsLocked(time.Now().UTC())
	m.recordLocked(Event{Type: "scenario_changed", Message: scenario.Name})
	return nil
}

func lookupScenario(name string) (Scenario, bool) {
	switch name {
	case ScenarioNormal, ScenarioNoSignal, ScenarioBadM3U, ScenarioTunerBusy, ScenarioRTPStop, ScenarioSlowRTSP, ScenarioMalformedPSI, ScenarioRTPLoss, ScenarioRTPJitter, ScenarioContinuityErrors, ScenarioEPGGap, ScenarioEPGMismatch, ScenarioEPGStale, ScenarioSignalDegraded, ScenarioLockLoss, ScenarioSlowLock:
		return scenarioByName(name), true
	default:
		return Scenario{}, false
	}
}

func SupportedScenarios() []Scenario {
	names := []string{
		ScenarioNormal,
		ScenarioNoSignal,
		ScenarioBadM3U,
		ScenarioTunerBusy,
		ScenarioRTPStop,
		ScenarioSlowRTSP,
		ScenarioMalformedPSI,
		ScenarioRTPLoss,
		ScenarioRTPJitter,
		ScenarioContinuityErrors,
		ScenarioEPGGap,
		ScenarioEPGMismatch,
		ScenarioEPGStale,
		ScenarioSignalDegraded,
		ScenarioLockLoss,
		ScenarioSlowLock,
	}
	scenarios := make([]Scenario, 0, len(names))
	for _, name := range names {
		scenarios = append(scenarios, scenarioByName(name))
	}
	return scenarios
}

func (s Scenario) AppliesTo(service Service, mux Mux) bool {
	if s.ServiceID != "" && s.ServiceID != service.ID {
		return false
	}
	if s.MuxID != "" && s.MuxID != mux.ID {
		return false
	}
	return true
}

func (s Scenario) SupportsTarget() bool {
	switch s.Name {
	case ScenarioNoSignal, ScenarioRTPStop, ScenarioMalformedPSI, ScenarioRTPLoss, ScenarioRTPJitter, ScenarioContinuityErrors, ScenarioEPGGap, ScenarioSignalDegraded, ScenarioLockLoss, ScenarioSlowLock:
		return true
	default:
		return false
	}
}

func scenarioByName(name string) Scenario {
	switch name {
	case ScenarioNoSignal:
		return Scenario{Name: ScenarioNoSignal, Description: "Reject valid RTSP SETUP requests with a simulated no-signal condition."}
	case ScenarioBadM3U:
		return Scenario{Name: ScenarioBadM3U, Description: "Return deliberately malformed channel list content from /channels.m3u."}
	case ScenarioTunerBusy:
		return Scenario{Name: ScenarioTunerBusy, Description: "Reject valid RTSP SETUP requests with a simulated tuner-busy condition before allocation."}
	case ScenarioRTPStop:
		return Scenario{Name: ScenarioRTPStop, Description: "Start RTP after PLAY, then stop sending packets after a short deterministic burst."}
	case ScenarioSlowRTSP:
		return Scenario{Name: ScenarioSlowRTSP, Description: "Delay RTSP responses to exercise client timeout and retry handling."}
	case ScenarioMalformedPSI:
		return Scenario{Name: ScenarioMalformedPSI, Description: "Corrupt PAT/PMT table headers while preserving RTP and MPEG-TS packet framing."}
	case ScenarioRTPLoss:
		return Scenario{Name: ScenarioRTPLoss, Description: "Drop a deterministic subset of RTP packets after PLAY."}
	case ScenarioRTPJitter:
		return Scenario{Name: ScenarioRTPJitter, Description: "Apply deterministic timing jitter to RTP packet delivery."}
	case ScenarioContinuityErrors:
		return Scenario{Name: ScenarioContinuityErrors, Description: "Corrupt MPEG-TS continuity counters while preserving packet framing."}
	case ScenarioEPGGap:
		return Scenario{Name: ScenarioEPGGap, Description: "Remove a deterministic XMLTV programme window for EPG gap handling."}
	case ScenarioEPGMismatch:
		return Scenario{Name: ScenarioEPGMismatch, Description: "Return XMLTV with one channel id that does not match the M3U tvg-id."}
	case ScenarioEPGStale:
		return Scenario{Name: ScenarioEPGStale, Description: "Return XMLTV with a stale Last-Modified timestamp relative to the lab clock."}
	case ScenarioSignalDegraded:
		return Scenario{Name: ScenarioSignalDegraded, Description: "Expose degraded deterministic RF frontend telemetry while allowing RTSP setup and playback."}
	case ScenarioLockLoss:
		return Scenario{Name: ScenarioLockLoss, Description: "Expose lost-lock deterministic RF frontend telemetry while keeping lab control paths deterministic."}
	case ScenarioSlowLock:
		return Scenario{Name: ScenarioSlowLock, Description: "Expose a slow frontend lock acquisition state and deterministic lock delay telemetry."}
	default:
		return Scenario{Name: ScenarioNormal, Description: "Normal SAT>IP simulator behavior."}
	}
}

func (m *Manager) allocateTunerLocked(muxID string) (int, error) {
	for _, tuner := range m.tuners {
		if tuner.State == "tuned" && tuner.MuxID == muxID {
			return tuner.ID, nil
		}
	}
	for i := range m.tuners {
		if m.tuners[i].State == "idle" {
			m.tuners[i].State = "tuned"
			m.tuners[i].MuxID = muxID
			return m.tuners[i].ID, nil
		}
	}
	return 0, ErrNoTunerAvailable
}

func (m *Manager) addSessionToTunerLocked(tunerID int, sessionID string) {
	for i := range m.tuners {
		if m.tuners[i].ID == tunerID {
			m.tuners[i].Sessions = append(m.tuners[i].Sessions, sessionID)
			return
		}
	}
}

func (m *Manager) removeSessionFromTunerLocked(tunerID int, sessionID string) {
	for i := range m.tuners {
		if m.tuners[i].ID != tunerID {
			continue
		}
		var sessions []string
		for _, existing := range m.tuners[i].Sessions {
			if existing != sessionID {
				sessions = append(sessions, existing)
			}
		}
		m.tuners[i].Sessions = sessions
		if len(sessions) == 0 {
			m.tuners[i].State = "idle"
			m.tuners[i].MuxID = ""
			m.tuners[i].Frontend = idleFrontend()
		} else {
			m.recomputeTunerFrontendLocked(tunerID, time.Now().UTC())
		}
		return
	}
}

func (m *Manager) recomputeAllFrontendsLocked(now time.Time) {
	for _, tuner := range m.tuners {
		m.recomputeTunerFrontendLocked(tuner.ID, now)
	}
}

func (m *Manager) recomputeTunerFrontendLocked(tunerID int, now time.Time) {
	for i := range m.tuners {
		if m.tuners[i].ID != tunerID {
			continue
		}
		if m.tuners[i].State == "idle" || len(m.tuners[i].Sessions) == 0 {
			m.tuners[i].Frontend = idleFrontend()
			return
		}
		frontend := lockedFrontend(now)
		for _, sessionID := range m.tuners[i].Sessions {
			session, ok := m.sessions[sessionID]
			if !ok {
				continue
			}
			service, ok := m.catalog.ServiceByID(session.ServiceID)
			if !ok {
				continue
			}
			mux, ok := m.catalog.MuxByID(session.MuxID)
			if !ok {
				continue
			}
			if m.scenario.AppliesTo(service, mux) {
				frontend = frontendForScenario(m.scenario, service, mux, now)
				break
			}
		}
		m.tuners[i].Frontend = frontend
		return
	}
}

func frontendForScenario(scenario Scenario, service Service, mux Mux, now time.Time) TunerFrontend {
	if !scenario.AppliesTo(service, mux) {
		return lockedFrontend(now)
	}
	switch scenario.Name {
	case ScenarioSignalDegraded:
		return TunerFrontend{State: FrontendDegraded, SignalStrength: 42, SNRDB: 6.5, BER: 0.00025, PER: 0.02, LockMS: 250, LastLockChange: &now}
	case ScenarioLockLoss:
		return TunerFrontend{State: FrontendLost, SignalStrength: 0, SNRDB: 0, BER: 1, PER: 1, LockMS: 250, LastLockChange: &now}
	case ScenarioSlowLock:
		return TunerFrontend{State: FrontendTuning, SignalStrength: 55, SNRDB: 8, BER: 0.0001, PER: 0.01, LockMS: 1200, LastLockChange: &now}
	default:
		return lockedFrontend(now)
	}
}

func lockedFrontend(now time.Time) TunerFrontend {
	return TunerFrontend{State: FrontendLocked, SignalStrength: 88, SNRDB: 13.5, BER: 0, PER: 0, LockMS: 250, LastLockChange: &now}
}

func idleFrontend() TunerFrontend {
	return TunerFrontend{State: FrontendIdle}
}

func (m *Manager) record(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordLocked(event)
}

func (m *Manager) recordLocked(event Event) {
	event.At = time.Now().UTC()
	m.events = append(m.events, event)
	if len(m.events) > 200 {
		m.events = append([]Event(nil), m.events[len(m.events)-200:]...)
	}
}
