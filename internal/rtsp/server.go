package rtsp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/epg"
	"github.com/e12media/satip-lab/internal/lab"
	"github.com/e12media/satip-lab/internal/ts"
	"github.com/e12media/satip-lab/internal/vendorprofile"
)

var clientPortPattern = regexp.MustCompile(`(?i)client_port=(\d+)-(\d+)`)
var destinationPattern = regexp.MustCompile(`(?i)destination=([^;]+)`)
var interleavedPattern = regexp.MustCompile(`(?i)interleaved=(\d+)-(\d+)`)

const slowRTSPDelay = 250 * time.Millisecond
const coldBootDelay = 750 * time.Millisecond
const delayedPSIStartupDelay = 80 * time.Millisecond
const rtspSessionTimeout = 60 * time.Second

type Server struct {
	cfg           config.Config
	vendorProfile vendorprofile.Profile
	streamSource  *ts.Source
	lab           *lab.Manager

	listener  net.Listener
	sessions  map[string]*session
	sessionMu sync.Mutex
	nextID    int
	stopCh    chan struct{}
	stopOnce  sync.Once
}

func NewServer(cfg config.Config, streamSource *ts.Source, labManager *lab.Manager) *Server {
	if labManager == nil {
		labManager = lab.NewManager(lab.DefaultCatalog(), cfg.TunerCount)
	}
	return &Server{
		cfg:           cfg,
		vendorProfile: cfg.CompatibilityProfile(),
		streamSource:  streamSource,
		lab:           labManager,
		sessions:      make(map[string]*session),
		stopCh:        make(chan struct{}),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.cfg.BindAddress, s.cfg.RTSPPort))
	if err != nil {
		return err
	}
	s.listener = ln
	s.startSessionReaper(5*time.Second, time.Now)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handleConn(conn)
		}
	}()
	return nil
}

func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.Reset()

	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *Server) Reset() {
	s.sessionMu.Lock()
	for _, sess := range s.sessions {
		sess.stopStreaming()
	}
	s.sessions = make(map[string]*session)
	s.sessionMu.Unlock()
}

func (s *Server) handleConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	state := newConnectionState()
	defer func() {
		s.closeConnectionSessions(state)
		_ = conn.Close()
	}()

	for {
		req, err := readRequest(reader)
		if err != nil {
			return
		}
		resp := s.handleRequestWithState(conn, req, state)
		state.writeMu.Lock()
		_, err = conn.Write([]byte(resp))
		state.writeMu.Unlock()
		if err != nil {
			return
		}
		state.runAfterWrite()
	}
}

type connectionState struct {
	described  bool
	writeMu    sync.Mutex
	sessionIDs map[string]struct{}
	afterWrite []func()
}

func newConnectionState() *connectionState {
	return &connectionState{sessionIDs: make(map[string]struct{})}
}

func (c *connectionState) rememberSession(sessionID string) {
	if c == nil {
		return
	}
	if c.sessionIDs == nil {
		c.sessionIDs = make(map[string]struct{})
	}
	c.sessionIDs[sessionID] = struct{}{}
}

func (c *connectionState) forgetSession(sessionID string) {
	if c == nil || c.sessionIDs == nil {
		return
	}
	delete(c.sessionIDs, sessionID)
}

func (c *connectionState) afterResponse(fn func()) {
	if c == nil || fn == nil {
		return
	}
	c.afterWrite = append(c.afterWrite, fn)
}

func (c *connectionState) runAfterWrite() {
	if c == nil || len(c.afterWrite) == 0 {
		return
	}
	callbacks := c.afterWrite
	c.afterWrite = nil
	for _, callback := range callbacks {
		callback()
	}
}

func (s *Server) handleRequest(conn net.Conn, req request) string {
	return s.handleRequestWithState(conn, req, &connectionState{})
}

func (s *Server) handleRequestWithState(conn net.Conn, req request, state *connectionState) string {
	cseq := req.headers["cseq"]
	if cseq == "" {
		cseq = "0"
	}
	s.expireSessions(time.Now().UTC())
	s.touchRequestSession(req, time.Now().UTC())

	if req.method == "DESCRIBE" && state != nil {
		state.described = true
	}
	if s.vendorProfile.RequireDescribeBeforeSetup && req.method == "SETUP" && (state == nil || !state.described) {
		return buildResponse(cseq, "455 Method Not Valid in This State", []string{
			"Reason: DESCRIBE required before SETUP",
		})
	}
	if s.cfg.Scenario == config.ScenarioTunerBusy && req.method == "SETUP" {
		return buildResponse(cseq, s.vendorProfile.TunerBusyStatus, []string{
			"Reason: tuner busy",
		})
	}
	switch s.lab.Scenario().Name {
	case lab.ScenarioSlowRTSP:
		time.Sleep(slowRTSPDelay)
	case lab.ScenarioColdBoot:
		time.Sleep(coldBootDelay)
	}

	switch req.method {
	case "OPTIONS":
		return buildResponse(cseq, "200 OK", []string{
			"Public: OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER",
		})
	case "DESCRIBE":
		return s.handleDescribe(cseq)
	case "SETUP":
		return s.handleSetupWithState(conn, req, cseq, state)
	case "PLAY":
		return s.handlePlayWithState(req, cseq, state)
	case "PAUSE":
		return s.handlePause(req, cseq)
	case "TEARDOWN":
		return s.handleTeardownWithState(req, cseq, state)
	case "GET_PARAMETER":
		return s.handleGetParameter(req, cseq)
	default:
		return buildResponse(cseq, "501 Not Implemented", nil)
	}
}

func (s *Server) handleDescribe(cseq string) string {
	body := strings.Join([]string{
		"v=0",
		fmt.Sprintf("o=- 0 0 IN IP4 %s", s.cfg.PublicHost),
		"s=SAT>IP Lab Server",
		"t=0 0",
		"a=control:*",
		"m=video 0 RTP/AVP 33",
		"a=rtpmap:33 MP2T/90000",
		"a=control:stream=0",
		"",
	}, "\r\n")
	return buildResponseWithBody(cseq, "200 OK", []string{
		"Content-Type: application/sdp",
	}, body)
}

func (s *Server) handleSetup(conn net.Conn, req request, cseq string) string {
	return s.handleSetupWithState(conn, req, cseq, nil)
}

func (s *Server) handleSetupWithState(conn net.Conn, req request, cseq string, state *connectionState) string {
	s.sessionMu.Lock()
	s.nextID++
	sessionID := fmt.Sprintf("%08d", s.nextID)
	s.sessionMu.Unlock()

	transport := parseTransport(req.headers["transport"])
	if transport.invalid {
		return buildResponse(cseq, "461 Unsupported Transport", []string{"Reason: invalid interleaved channel"})
	}
	remote := conn.RemoteAddr().(*net.TCPAddr)
	rawQuery := tuningQueryFromURI(req.uri)
	setup, err := s.lab.Setup(sessionID, rawQuery, remote.IP.String())
	if err != nil {
		switch err {
		case lab.ErrNoTunerAvailable:
			return buildResponse(cseq, s.vendorProfile.TunerBusyStatus, []string{"Reason: tuner busy"})
		case lab.ErrNoSignal:
			return buildResponse(cseq, "503 Service Unavailable", []string{"Reason: no signal"})
		case lab.ErrTunerWedged:
			return buildResponse(cseq, "503 Service Unavailable", []string{"Reason: tuner wedged"})
		case lab.ErrServiceNotFound:
			return buildResponse(cseq, "404 Not Found", []string{"Reason: service not found"})
		default:
			return buildResponse(cseq, "400 Bad Request", []string{"Reason: invalid tuning"})
		}
	}
	sess := &session{
		id:            sessionID,
		clientIP:      transport.destination(remote.IP),
		clientRTPPort: transport.rtpPort,
		clientRTCPort: transport.rtcpPort,
		transport:     transport.mode,
		rtpChannel:    transport.rtpChannel,
		rtspConn:      conn,
		onStreamError: s.closeSessionAfterConnectionLoss,
		service:       setup.Service,
	}
	if state != nil {
		sess.rtspWriteMu = &state.writeMu
		state.rememberSession(sessionID)
	}
	s.sessionMu.Lock()
	s.sessions[sessionID] = sess
	s.sessionMu.Unlock()

	if transport.mode == transportInterleaved {
		return buildResponse(cseq, "200 OK", []string{
			s.setupSessionHeader(sessionID),
			fmt.Sprintf(
				"%s: RTP/AVP/TCP;unicast;interleaved=%d-%d;source=%s",
				s.vendorProfile.TransportHeader,
				transport.rtpChannel, transport.rtcpChannel, s.cfg.PublicHost,
			),
		})
	}

	return buildResponse(cseq, "200 OK", []string{
		s.setupSessionHeader(sessionID),
		fmt.Sprintf(
			"%s: RTP/AVP;unicast;destination=%s;source=%s;client_port=%d-%d;server_port=5000-5001",
			s.vendorProfile.TransportHeader,
			sess.clientIP.String(), s.cfg.PublicHost, transport.rtpPort, transport.rtcpPort,
		),
	})
}

func (s *Server) handlePlay(req request, cseq string) string {
	return s.handlePlayWithState(req, cseq, nil)
}

func (s *Server) handlePlayWithState(req request, cseq string, state *connectionState) string {
	sessionID := sessionIDFrom(req.headers["session"])
	if sessionID == "" {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}

	sess, ok := s.sessionByID(sessionID)
	if !ok {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	if err := s.lab.UpdatePIDs(sessionID, tuningQueryFromURI(req.uri)); err != nil {
		return buildResponse(cseq, "400 Bad Request", []string{"Reason: invalid pid update"})
	}

	setup, err := s.lab.Play(sessionID)
	if err != nil {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	profile := ts.ServiceProfile{
		ID:        setup.Service.ID,
		Name:      setup.Service.Name,
		ServiceID: setup.Service.ServiceID,
		PMTPID:    setup.Service.PMTPID,
		VideoPID:  setup.Service.VideoPID,
		AudioPID:  setup.Service.AudioPID,
	}
	payloadProvider, err := s.playPayloadProvider(profile, setup.Service, setup.Mux)
	if err != nil {
		return buildResponse(cseq, "500 Internal Server Error", nil)
	}
	start := func() {
		sess.startStreaming(payloadProvider, NewRTPSender(), func() streamBehavior {
			return s.streamBehavior(setup.Service, setup.Mux)
		})
	}
	if state != nil && sess.transport == transportInterleaved {
		state.afterResponse(start)
	} else {
		start()
	}

	return buildResponse(cseq, "200 OK", s.sessionHeaders(sessionID))
}

func (s *Server) closeSessionAfterConnectionLoss(sessionID string) {
	s.stopAndDeleteSession(sessionID)
	s.lab.Teardown(sessionID)
}

func (s *Server) closeConnectionSessions(state *connectionState) {
	if state == nil {
		return
	}
	for sessionID := range state.sessionIDs {
		sess, ok := s.sessionByID(sessionID)
		if !ok || sess.transport != transportInterleaved {
			state.forgetSession(sessionID)
			continue
		}
		s.closeSessionAfterConnectionLoss(sessionID)
		state.forgetSession(sessionID)
	}
}

func (s *Server) handlePause(req request, cseq string) string {
	sessionID := sessionIDFrom(req.headers["session"])
	if sessionID == "" {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}

	sess, ok := s.sessionByID(sessionID)
	if !ok {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	sess.stopStreaming()
	if err := s.lab.Pause(sessionID); err != nil {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	return buildResponse(cseq, "200 OK", s.sessionHeaders(sessionID))
}

func (s *Server) streamBehavior(service lab.Service, mux lab.Mux) streamBehavior {
	scenario := s.lab.Scenario()
	if !scenario.AppliesTo(service, mux) {
		return streamBehavior{}
	}
	switch scenario.Name {
	case lab.ScenarioRTPStop:
		return streamBehavior{packetLimit: 3}
	case lab.ScenarioRTPLoss:
		return streamBehavior{dropEvery: 3}
	case lab.ScenarioRTPJitter:
		return streamBehavior{jitterEvery: 3, jitterDelay: 40 * time.Millisecond}
	case lab.ScenarioRTPBlackhole:
		return streamBehavior{dropAll: true}
	case lab.ScenarioDelayedPSI:
		return streamBehavior{startupDelay: delayedPSIStartupDelay}
	default:
		return streamBehavior{}
	}
}

func (s *Server) playPayload(profile ts.ServiceProfile, service lab.Service, mux lab.Mux) ([]byte, error) {
	return s.playPayloadForScenario(profile, service, mux, s.lab.Scenario())
}

func (s *Server) playPayloadProvider(profile ts.ServiceProfile, service lab.Service, mux lab.Mux) (streamPayloadProvider, error) {
	var mu sync.Mutex
	cache := make(map[string][]byte)
	lastKey := ""

	load := func(scenario lab.Scenario) ([]byte, error) {
		key := streamPayloadKey(scenario, service, mux)
		mu.Lock()
		if payload, ok := cache[key]; ok {
			mu.Unlock()
			return payload, nil
		}
		mu.Unlock()

		payload, err := s.playPayloadForScenario(profile, service, mux, scenario)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		cache[key] = payload
		lastKey = key
		mu.Unlock()
		return payload, nil
	}

	if _, err := load(s.lab.Scenario()); err != nil {
		return nil, err
	}
	return func() []byte {
		scenario := s.lab.Scenario()
		payload, err := load(scenario)
		if err == nil {
			return payload
		}
		mu.Lock()
		defer mu.Unlock()
		return cache[lastKey]
	}, nil
}

func streamPayloadKey(scenario lab.Scenario, service lab.Service, mux lab.Mux) string {
	if !scenario.AppliesTo(service, mux) {
		return lab.ScenarioNormal
	}
	switch scenario.Name {
	case lab.ScenarioMalformedPSI, lab.ScenarioContinuityErrors, lab.ScenarioEPGGap:
		return fmt.Sprintf("%s|%s|%s|%d", scenario.Name, scenario.ServiceID, scenario.MuxID, scenario.DurationMin)
	default:
		return lab.ScenarioNormal
	}
}

func (s *Server) playPayloadForScenario(profile ts.ServiceProfile, service lab.Service, mux lab.Mux, scenario lab.Scenario) ([]byte, error) {
	clock, err := epg.ParseClock(s.cfg.EPGClock)
	if err != nil {
		return nil, err
	}
	eit := ts.EITOptions{Now: clock.Now}
	if scenario.Name == lab.ScenarioEPGGap && scenario.AppliesTo(service, mux) {
		eit.Suppress = true
	}
	payload, err := s.streamSource.LoadServicePayloadWithOptions(profile, eit)
	if err != nil {
		return nil, err
	}
	if !scenario.AppliesTo(service, mux) {
		return payload, nil
	}
	switch scenario.Name {
	case lab.ScenarioMalformedPSI:
		return ts.MalformedPSI(payload), nil
	case lab.ScenarioContinuityErrors:
		return ts.ContinuityCounterErrors(payload), nil
	default:
		return payload, nil
	}
}

func (s *Server) handleTeardown(req request, cseq string) string {
	return s.handleTeardownWithState(req, cseq, nil)
}

func (s *Server) handleTeardownWithState(req request, cseq string, state *connectionState) string {
	sessionID := sessionIDFrom(req.headers["session"])
	if sessionID != "" {
		s.stopAndDeleteSession(sessionID)
		s.lab.Teardown(sessionID)
		if state != nil {
			state.forgetSession(sessionID)
		}
	}
	if sessionID != "" {
		return buildResponse(cseq, "200 OK", s.sessionHeaders(sessionID))
	}
	return buildResponse(cseq, "200 OK", nil)
}

func (s *Server) handleGetParameter(req request, cseq string) string {
	sessionID := sessionIDFrom(req.headers["session"])
	if sessionID == "" {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	if _, ok := s.sessionByID(sessionID); !ok {
		return buildResponse(cseq, "454 Session Not Found", nil)
	}
	return buildResponse(cseq, "200 OK", s.sessionHeaders(sessionID))
}

func (s *Server) touchRequestSession(req request, now time.Time) {
	sessionID := sessionIDFrom(req.headers["session"])
	if sessionID == "" {
		return
	}
	if _, ok := s.sessionByID(sessionID); !ok {
		return
	}
	_ = s.lab.Touch(sessionID, now)
}

func (s *Server) startSessionReaper(interval time.Duration, now func() time.Time) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.expireSessions(now())
			}
		}
	}()
}

func (s *Server) expireSessions(now time.Time) {
	expired := s.lab.ExpireSessions(now.UTC(), rtspSessionTimeout)
	if len(expired) == 0 {
		return
	}
	expiredSet := make(map[string]struct{}, len(expired))
	for _, sessionID := range expired {
		expiredSet[sessionID] = struct{}{}
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	for sessionID := range expiredSet {
		if sess, ok := s.sessions[sessionID]; ok {
			sess.stopStreaming()
			delete(s.sessions, sessionID)
		}
	}
}

func (s *Server) sessionByID(sessionID string) (*session, bool) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	sess, ok := s.sessions[sessionID]
	return sess, ok
}

func (s *Server) stopAndDeleteSession(sessionID string) {
	s.sessionMu.Lock()
	sess, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
	}
	s.sessionMu.Unlock()
	if ok {
		sess.stopStreaming()
	}
}

type session struct {
	id            string
	clientIP      net.IP
	clientRTPPort int
	clientRTCPort int
	transport     transportMode
	rtpChannel    int
	rtspConn      net.Conn
	rtspWriteMu   *sync.Mutex
	onStreamError func(string)
	service       lab.Service

	streamMu sync.Mutex
	udpConn  *net.UDPConn
	stopCh   chan struct{}
}

type streamBehavior struct {
	packetLimit  int
	dropEvery    int
	dropAll      bool
	startupDelay time.Duration
	jitterEvery  int
	jitterDelay  time.Duration
}

type streamPayloadProvider func() []byte

type streamBehaviorProvider func() streamBehavior

func (b streamBehavior) shouldDrop(packetNumber int) bool {
	if b.dropAll {
		return true
	}
	return b.dropEvery > 0 && packetNumber%b.dropEvery == 0
}

func (b streamBehavior) jitterFor(packetNumber int) time.Duration {
	if b.jitterEvery > 0 && packetNumber%b.jitterEvery == 0 {
		return b.jitterDelay
	}
	return 0
}

func (s *session) startStreaming(payloadProvider streamPayloadProvider, sender *RTPSender, behaviorProvider streamBehaviorProvider) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	s.stopStreamingLocked()

	if s.transport == transportInterleaved {
		s.startInterleavedStreamingLocked(payloadProvider, sender, behaviorProvider)
		return
	}
	s.startUDPStreamingLocked(payloadProvider, sender, behaviorProvider)
}

func (s *session) startUDPStreamingLocked(payloadProvider streamPayloadProvider, sender *RTPSender, behaviorProvider streamBehaviorProvider) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return
	}
	s.udpConn = conn
	stopCh := make(chan struct{})
	s.stopCh = stopCh

	dest := &net.UDPAddr{IP: s.clientIP, Port: s.clientRTPPort}
	source := &ts.Source{}

	go func() {
		offset := 0
		behaviorSent := 0
		packetNumber := 0
		behaviorPacketNumber := 0
		var lastBehavior streamBehavior
		var behaviorStartedAt time.Time
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				behavior := streamBehavior{}
				if behaviorProvider != nil {
					behavior = behaviorProvider()
				}
				if behavior != lastBehavior {
					behaviorSent = 0
					behaviorPacketNumber = 0
					lastBehavior = behavior
					behaviorStartedAt = time.Now()
				}
				if behavior.startupDelay > 0 && time.Since(behaviorStartedAt) < behavior.startupDelay {
					continue
				}
				payload := payloadProvider()
				chunk, next := source.ChunkAt(payload, offset)
				if len(chunk) > 0 {
					packetNumber++
					behaviorPacketNumber++
					if !behavior.shouldDrop(behaviorPacketNumber) {
						if jitter := behavior.jitterFor(packetNumber); jitter > 0 {
							time.Sleep(jitter)
						}
						_ = sender.Send(conn, dest, chunk)
						behaviorSent++
						if behavior.packetLimit > 0 && behaviorSent >= behavior.packetLimit {
							s.finishStreaming(conn, stopCh)
							return
						}
					} else {
						sender.Skip()
					}
				}
				offset = next
			}
		}
	}()
}

func (s *session) startInterleavedStreamingLocked(payloadProvider streamPayloadProvider, sender *RTPSender, behaviorProvider streamBehaviorProvider) {
	if s.rtspConn == nil {
		return
	}
	stopCh := make(chan struct{})
	s.stopCh = stopCh
	source := &ts.Source{}
	writeMu := s.rtspWriteMu
	if writeMu == nil {
		writeMu = &sync.Mutex{}
		s.rtspWriteMu = writeMu
	}

	go func() {
		offset := 0
		behaviorSent := 0
		packetNumber := 0
		behaviorPacketNumber := 0
		var lastBehavior streamBehavior
		var behaviorStartedAt time.Time
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				behavior := streamBehavior{}
				if behaviorProvider != nil {
					behavior = behaviorProvider()
				}
				if behavior != lastBehavior {
					behaviorSent = 0
					behaviorPacketNumber = 0
					lastBehavior = behavior
					behaviorStartedAt = time.Now()
				}
				if behavior.startupDelay > 0 && time.Since(behaviorStartedAt) < behavior.startupDelay {
					continue
				}
				payload := payloadProvider()
				chunk, next := source.ChunkAt(payload, offset)
				if len(chunk) > 0 {
					packetNumber++
					behaviorPacketNumber++
					if !behavior.shouldDrop(behaviorPacketNumber) {
						if jitter := behavior.jitterFor(packetNumber); jitter > 0 {
							time.Sleep(jitter)
						}
						frame := interleavedFrame(s.rtpChannel, sender.Packet(chunk))
						writeMu.Lock()
						_, err := s.rtspConn.Write(frame)
						writeMu.Unlock()
						if err != nil {
							s.finishInterleavedStreaming(stopCh)
							if s.onStreamError != nil {
								s.onStreamError(s.id)
							}
							return
						}
						behaviorSent++
						if behavior.packetLimit > 0 && behaviorSent >= behavior.packetLimit {
							s.finishInterleavedStreaming(stopCh)
							return
						}
					} else {
						sender.Skip()
					}
				}
				offset = next
			}
		}
	}()
}

func (s *session) streamingActive() bool {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	return s.udpConn != nil || s.stopCh != nil
}

func (s *session) finishStreaming(conn *net.UDPConn, stopCh chan struct{}) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.stopCh != stopCh {
		return
	}
	if s.udpConn == conn {
		_ = s.udpConn.Close()
		s.udpConn = nil
	}
	s.stopCh = nil
}

func (s *session) finishInterleavedStreaming(stopCh chan struct{}) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.stopCh != stopCh {
		return
	}
	s.stopCh = nil
}

func (s *session) stopStreaming() {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	s.stopStreamingLocked()
}

func (s *session) stopStreamingLocked() {
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
	if s.udpConn != nil {
		_ = s.udpConn.Close()
		s.udpConn = nil
	}
}

type request struct {
	method  string
	uri     string
	headers map[string]string
}

func readRequest(reader *bufio.Reader) (request, error) {
	var buffer strings.Builder
	for {
		prefix, err := reader.Peek(1)
		if err != nil {
			return request{}, err
		}
		if prefix[0] == '$' {
			if err := discardInterleavedFrame(reader); err != nil {
				return request{}, err
			}
			continue
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return request{}, err
		}
		buffer.WriteString(line)
		if strings.HasSuffix(buffer.String(), "\r\n\r\n") {
			req := parseRequest(buffer.String())
			contentLength, _ := strconv.Atoi(req.headers["content-length"])
			if contentLength > 0 {
				if _, err := io.CopyN(io.Discard, reader, int64(contentLength)); err != nil {
					return request{}, err
				}
			}
			return req, nil
		}
	}
}

func discardInterleavedFrame(reader *bufio.Reader) error {
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return err
	}
	length := int(header[2])<<8 | int(header[3])
	if length > 0 {
		_, err := io.CopyN(io.Discard, reader, int64(length))
		return err
	}
	return nil
}

func parseRequest(raw string) request {
	lines := strings.Split(raw, "\r\n")
	parts := strings.Split(lines[0], " ")
	headers := make(map[string]string)
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		headers[key] = val
	}
	uri := "/"
	if len(parts) > 1 {
		uri = parts[1]
	}
	return request{method: parts[0], uri: uri, headers: headers}
}

func buildResponse(cseq, status string, extra []string) string {
	lines := []string{
		"RTSP/1.0 " + status,
		"CSeq: " + cseq,
	}
	lines = append(lines, extra...)
	lines = append(lines, "", "")
	return strings.Join(lines, "\r\n")
}

func buildResponseWithBody(cseq, status string, extra []string, body string) string {
	headers := append([]string{}, extra...)
	headers = append(headers, "Content-Length: "+strconv.Itoa(len(body)))
	return buildResponse(cseq, status, headers) + body
}

type transportSpec struct {
	destinationIP net.IP
	rtpPort       int
	rtcpPort      int
	mode          transportMode
	rtpChannel    int
	rtcpChannel   int
	invalid       bool
}

type transportMode string

const (
	transportUDP         transportMode = "udp"
	transportInterleaved transportMode = "interleaved"
)

func parseTransport(header string) transportSpec {
	spec := transportSpec{rtpPort: 5000, rtcpPort: 5001, mode: transportUDP, rtpChannel: 0, rtcpChannel: 1}
	if header == "" {
		return spec
	}
	if strings.Contains(strings.ToLower(header), "rtp/avp/tcp") {
		spec.mode = transportInterleaved
	}
	if m := clientPortPattern.FindStringSubmatch(header); m != nil {
		spec.rtpPort, _ = strconv.Atoi(m[1])
		spec.rtcpPort, _ = strconv.Atoi(m[2])
	}
	if m := interleavedPattern.FindStringSubmatch(header); m != nil {
		spec.rtpChannel, _ = strconv.Atoi(m[1])
		spec.rtcpChannel, _ = strconv.Atoi(m[2])
	}
	if spec.mode == transportInterleaved && (spec.rtpChannel < 0 || spec.rtpChannel > 255 || spec.rtcpChannel < 0 || spec.rtcpChannel > 255) {
		spec.invalid = true
	}
	if m := destinationPattern.FindStringSubmatch(header); m != nil {
		spec.destinationIP = net.ParseIP(strings.TrimSpace(m[1]))
	}
	return spec
}

func (t transportSpec) destination(remoteIP net.IP) net.IP {
	if t.destinationIP != nil {
		return t.destinationIP
	}
	return remoteIP
}

func (s *Server) setupSessionHeader(sessionID string) string {
	header := s.vendorProfile.SessionHeader + ": " + sessionID
	if s.vendorProfile.IncludeSetupTimeout {
		header += fmt.Sprintf(";timeout=%d", int(rtspSessionTimeout/time.Second))
	}
	return header
}

func (s *Server) sessionHeaders(sessionID string) []string {
	return []string{s.vendorProfile.SessionHeader + ": " + sessionID}
}

func sessionHeaders(sessionID string) []string {
	return vendorprofileHeaders(vendorprofile.ForName(vendorprofile.NameSpec), sessionID)
}

func vendorprofileHeaders(profile vendorprofile.Profile, sessionID string) []string {
	return []string{profile.SessionHeader + ": " + sessionID}
}

func tuningQueryFromURI(uri string) string {
	idx := strings.Index(uri, "?")
	if idx < 0 || idx == len(uri)-1 {
		return ""
	}
	return uri[idx+1:]
}

func sessionIDFrom(header string) string {
	if header == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(header, ";")[0])
}

func interleavedFrame(channel int, packet []byte) []byte {
	frame := make([]byte, 4+len(packet))
	frame[0] = '$'
	frame[1] = byte(channel)
	frame[2] = byte(len(packet) >> 8)
	frame[3] = byte(len(packet))
	copy(frame[4:], packet)
	return frame
}
