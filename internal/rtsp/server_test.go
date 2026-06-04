package rtsp

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/lab"
	"github.com/e12media/satip-lab/internal/ts"
	"github.com/e12media/satip-lab/internal/vendorprofile"
)

func TestParseTransportUsesDestinationAndClientPorts(t *testing.T) {
	transport := parseTransport("RTP/AVP;unicast;destination=192.0.2.55;client_port=6000-6001")

	if !transport.destinationIP.Equal(net.ParseIP("192.0.2.55")) {
		t.Fatalf("destination IP: got %v", transport.destinationIP)
	}
	if transport.rtpPort != 6000 || transport.rtcpPort != 6001 {
		t.Fatalf("client ports: got %d-%d", transport.rtpPort, transport.rtcpPort)
	}
}

func TestParseTransportIsCaseInsensitive(t *testing.T) {
	transport := parseTransport("RTP/AVP;unicast;Destination=192.0.2.56;Client_Port=6100-6101")

	if !transport.destinationIP.Equal(net.ParseIP("192.0.2.56")) {
		t.Fatalf("destination IP: got %v", transport.destinationIP)
	}
	if transport.rtpPort != 6100 || transport.rtcpPort != 6101 {
		t.Fatalf("client ports: got %d-%d", transport.rtpPort, transport.rtcpPort)
	}
}

func TestParseTransportFallsBackToRemoteAddress(t *testing.T) {
	remote := net.ParseIP("198.51.100.10")
	transport := parseTransport("RTP/AVP;unicast;client_port=7000-7001")

	if got := transport.destination(remote); !got.Equal(remote) {
		t.Fatalf("destination fallback: got %v", got)
	}
}

func TestPlayAndTeardownIncludeSessionHeader(t *testing.T) {
	resp := buildResponse("4", "200 OK", sessionHeaders("00000042"))

	if !strings.Contains(resp, "Session: 00000042") {
		t.Fatalf("missing session header: %q", resp)
	}
	if !strings.HasSuffix(resp, "\r\n\r\n") {
		t.Fatalf("response must end with CRLFCRLF: %q", resp)
	}
}

func TestOptionsAdvertisesCompatibilityMethods(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "OPTIONS", headers: map[string]string{"cseq": "1"}},
	)

	if !strings.Contains(resp, "Public: OPTIONS, DESCRIBE, SETUP, PLAY, PAUSE, TEARDOWN, GET_PARAMETER") {
		t.Fatalf("missing compatibility methods: %s", resp)
	}
}

func TestDescribeReturnsMinimalSDP(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "198.51.100.1"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "DESCRIBE", uri: "rtsp://198.51.100.1/", headers: map[string]string{"cseq": "2"}},
	)

	if !strings.Contains(resp, "RTSP/1.0 200 OK") {
		t.Fatalf("DESCRIBE failed: %s", resp)
	}
	if !strings.Contains(resp, "Content-Type: application/sdp") {
		t.Fatalf("missing SDP content type: %s", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nv=0\r\n") || !strings.Contains(resp, "s=SAT>IP Lab Server\r\n") {
		t.Fatalf("missing SDP body: %s", resp)
	}
	if !strings.Contains(resp, "m=video 0 RTP/AVP 33\r\n") || !strings.Contains(resp, "a=rtpmap:33 MP2T/90000\r\n") {
		t.Fatalf("missing MP2T media description: %s", resp)
	}
}

func TestNewServerResolvesSpecVendorProfile(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1", VendorProfile: "spec"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))

	if server.vendorProfile.Name != "spec" {
		t.Fatalf("vendor profile: got %q", server.vendorProfile.Name)
	}
	if server.vendorProfile.SessionIDFormat != vendorprofile.SessionIDNumeric {
		t.Fatalf("session id format: got %q", server.vendorProfile.SessionIDFormat)
	}
}

func TestSpecVendorProfileAllowsSetupWithoutDescribeAndUsesStandardHeaders(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1", VendorProfile: "spec"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))
	req := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120",
		headers: map[string]string{"transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		req,
	)

	if !strings.Contains(resp, "RTSP/1.0 200 OK") {
		t.Fatalf("SETUP without DESCRIBE failed: %s", resp)
	}
	if !strings.Contains(resp, "Session: 00000001;timeout=60") {
		t.Fatalf("missing spec Session header: %s", resp)
	}
	if !strings.Contains(resp, "Transport: RTP/AVP;unicast;") {
		t.Fatalf("missing spec Transport header: %s", resp)
	}
}

func TestSpecVendorProfileUses503ForStartupTunerBusy(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1", VendorProfile: "spec", Scenario: config.ScenarioTunerBusy}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))
	req := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120",
		headers: map[string]string{"cseq": "1", "transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		req,
	)

	if !strings.Contains(resp, "RTSP/1.0 503 Service Unavailable") {
		t.Fatalf("expected spec tuner busy 503: %s", resp)
	}
	if !strings.Contains(resp, "Reason: tuner busy") {
		t.Fatalf("expected tuner busy reason: %s", resp)
	}
}

func TestVendorProfileCanRequireDescribeBeforeSetup(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1", VendorProfile: "spec"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 1))
	server.vendorProfile.RequireDescribeBeforeSetup = true
	state := &connectionState{}
	conn := &fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}}
	setup := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120",
		headers: map[string]string{"cseq": "1", "transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}

	beforeDescribe := server.handleRequestWithState(conn, setup, state)
	if !strings.Contains(beforeDescribe, "RTSP/1.0 455 Method Not Valid in This State") {
		t.Fatalf("expected SETUP before DESCRIBE to be rejected: %s", beforeDescribe)
	}

	describe := server.handleRequestWithState(conn, request{method: "DESCRIBE", headers: map[string]string{"cseq": "2"}}, state)
	if !strings.Contains(describe, "RTSP/1.0 200 OK") {
		t.Fatalf("DESCRIBE failed: %s", describe)
	}

	afterDescribe := server.handleRequestWithState(conn, setup, state)
	if !strings.Contains(afterDescribe, "RTSP/1.0 200 OK") {
		t.Fatalf("expected SETUP after DESCRIBE to pass: %s", afterDescribe)
	}
}

func TestSetupRejectsUnknownTuning(t *testing.T) {
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, lab.NewManager(lab.DefaultCatalog(), 2))
	req := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=99999&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100",
		headers: map[string]string{"transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}

	resp := server.handleSetup(&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}}, req, "1")
	if !strings.Contains(resp, "404 Not Found") {
		t.Fatalf("expected 404 for unknown tuning: %s", resp)
	}
}

func TestSetupReturns503WhenTunersAreBusy(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	conn := &fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}}

	first := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102",
		headers: map[string]string{"transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}
	if resp := server.handleSetup(conn, first, "1"); !strings.Contains(resp, "200 OK") {
		t.Fatalf("first setup failed: %s", resp)
	}

	second := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120",
		headers: map[string]string{"transport": "RTP/AVP;unicast;client_port=6002-6003"},
	}
	if resp := server.handleSetup(conn, second, "2"); !strings.Contains(resp, "503 Service Unavailable") {
		t.Fatalf("expected tuner busy: %s", resp)
	}
}

func TestSetupReturns503WhenNoSignalScenarioIsActive(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioNoSignal); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	req := request{
		method:  "SETUP",
		uri:     "rtsp://127.0.0.1/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102",
		headers: map[string]string{"transport": "RTP/AVP;unicast;client_port=6000-6001"},
	}

	resp := server.handleSetup(&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}}, req, "1")
	if !strings.Contains(resp, "503 Service Unavailable") || !strings.Contains(resp, "no signal") {
		t.Fatalf("expected no signal 503, got: %s", resp)
	}
}

func TestSlowRTSPScenarioDelaysResponses(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioSlowRTSP); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	start := time.Now()

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "OPTIONS", headers: map[string]string{"cseq": "1"}},
	)

	if !strings.Contains(resp, "200 OK") {
		t.Fatalf("OPTIONS failed: %s", resp)
	}
	if elapsed := time.Since(start); elapsed < slowRTSPDelay {
		t.Fatalf("expected slow RTSP delay >= %s, got %s", slowRTSPDelay, elapsed)
	}
}

func TestReadRequestConsumesBody(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(
		"GET_PARAMETER rtsp://127.0.0.1/ RTSP/1.0\r\nCSeq: 2\r\nSession: 00000001\r\nContent-Length: 4\r\n\r\nping" +
			"OPTIONS rtsp://127.0.0.1/ RTSP/1.0\r\nCSeq: 3\r\n\r\n",
	))

	first, err := readRequest(reader)
	if err != nil {
		t.Fatal(err)
	}
	if first.method != "GET_PARAMETER" {
		t.Fatalf("first method: got %q", first.method)
	}
	second, err := readRequest(reader)
	if err != nil {
		t.Fatal(err)
	}
	if second.method != "OPTIONS" {
		t.Fatalf("second method after body consume: got %q", second.method)
	}
}

func TestGetParameterKeepsSessionAlive(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	sessionID := setupTestSession(t, server)
	before := manager.Status().Sessions[0].UpdatedAt
	time.Sleep(time.Millisecond)

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "GET_PARAMETER", headers: map[string]string{"cseq": "2", "session": sessionID}},
	)

	if !strings.Contains(resp, "200 OK") || !strings.Contains(resp, "Session: "+sessionID) {
		t.Fatalf("GET_PARAMETER failed: %s", resp)
	}
	if got := manager.Status().Sessions[0].State; got != "setup" {
		t.Fatalf("GET_PARAMETER should not change session state, got %q", got)
	}
	if !manager.Status().Sessions[0].UpdatedAt.After(before) {
		t.Fatal("GET_PARAMETER should refresh session activity")
	}
}

func TestPlayUpdatesSessionPIDs(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	sessionID := setupTestSession(t, server)

	resp := server.handlePlay(request{
		uri:     "rtsp://127.0.0.1/?addpids=8190&delpids=17",
		headers: map[string]string{"session": sessionID},
	}, "2")

	if !strings.Contains(resp, "200 OK") {
		t.Fatalf("PLAY with PID update failed: %s", resp)
	}
	if got := manager.Status().Sessions[0].PIDs; !sameInts(got, []int{0, 5100, 5101, 5102, 8190}) {
		t.Fatalf("updated session PIDs: got %#v", got)
	}
}

func TestOptionsWithSessionKeepsSessionAlive(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	sessionID := setupTestSession(t, server)
	before := manager.Status().Sessions[0].UpdatedAt
	time.Sleep(time.Millisecond)

	resp := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "OPTIONS", headers: map[string]string{"cseq": "2", "session": sessionID}},
	)

	if !strings.Contains(resp, "200 OK") {
		t.Fatalf("OPTIONS failed: %s", resp)
	}
	if !manager.Status().Sessions[0].UpdatedAt.After(before) {
		t.Fatal("OPTIONS with Session should refresh session activity")
	}
}

func TestPauseStopsStreamingAndKeepsSession(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()
	sessionID := setupTestSessionWithRTPPort(t, server, rtpConn.LocalAddr().(*net.UDPAddr).Port)

	play := server.handlePlay(request{headers: map[string]string{"session": sessionID}}, "2")
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}
	buf := make([]byte, 2048)
	_ = rtpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := rtpConn.ReadFromUDP(buf); err != nil {
		t.Fatalf("expected RTP before PAUSE: %v", err)
	}

	pause := server.handleRequest(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{method: "PAUSE", headers: map[string]string{"cseq": "3", "session": sessionID}},
	)
	if !strings.Contains(pause, "200 OK") || !strings.Contains(pause, "Session: "+sessionID) {
		t.Fatalf("PAUSE failed: %s", pause)
	}
	if got := manager.Status().Sessions[0].State; got != "paused" {
		t.Fatalf("session state after PAUSE: got %q", got)
	}
	_ = rtpConn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	if _, _, err := rtpConn.ReadFromUDP(buf); err == nil {
		t.Fatal("expected RTP stream to stop after PAUSE")
	}
}

func TestExpiredSessionStopsStreamingAndReleasesTuner(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()
	sessionID := setupTestSessionWithRTPPort(t, server, rtpConn.LocalAddr().(*net.UDPAddr).Port)

	play := server.handlePlay(request{headers: map[string]string{"session": sessionID}}, "2")
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}
	server.expireSessions(time.Now().Add(rtspSessionTimeout + time.Second))

	if len(manager.Status().Sessions) != 0 || manager.Status().Tuners[0].State != "idle" {
		t.Fatalf("expired session should release lab state: %+v", manager.Status())
	}
	_ = rtpConn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	if _, _, err := rtpConn.ReadFromUDP(make([]byte, 2048)); err == nil {
		t.Fatal("expected RTP stream to stop after session timeout")
	}
	resp := server.handleGetParameter(request{headers: map[string]string{"session": sessionID}}, "3")
	if !strings.Contains(resp, "454 Session Not Found") {
		t.Fatalf("expired RTSP session should be gone: %s", resp)
	}
}

func TestSessionReaperExpiresIdleSessionsWithoutTraffic(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	defer server.Stop()
	sessionID := setupTestSession(t, server)

	server.startSessionReaper(time.Millisecond, func() time.Time {
		return time.Now().UTC().Add(rtspSessionTimeout + time.Second)
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(manager.Status().Sessions) == 0 {
			if _, ok := server.sessionByID(sessionID); ok {
				t.Fatal("expired session remained in RTSP session map")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("session did not expire without traffic: %+v", manager.Status())
}

func TestStartStreamingStopsAfterPacketLimit(t *testing.T) {
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()

	sess := &session{
		clientIP:      net.ParseIP("127.0.0.1"),
		clientRTPPort: rtpConn.LocalAddr().(*net.UDPAddr).Port,
	}
	payload := make([]byte, 188)
	payload[0] = 0x47

	sess.startStreaming(payload, NewRTPSender(), streamBehavior{packetLimit: 2})
	defer sess.stopStreaming()

	buf := make([]byte, 2048)
	for i := 0; i < 2; i++ {
		_ = rtpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		if _, _, err := rtpConn.ReadFromUDP(buf); err != nil {
			t.Fatalf("packet %d: %v", i+1, err)
		}
	}

	_ = rtpConn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	if _, _, err := rtpConn.ReadFromUDP(buf); err == nil {
		t.Fatal("expected RTP stream to stop after packet limit")
	} else {
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Fatalf("expected read timeout after packet limit, got %v", err)
		}
	}
	if sess.streamingActive() {
		t.Fatal("expected RTP sender resources to be released after packet limit")
	}
}

func TestPlayUsesRTPStopScenarioPacketLimit(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioRTPStop); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()
	rtpPort := rtpConn.LocalAddr().(*net.UDPAddr).Port

	setup := server.handleSetup(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{
			method: "SETUP",
			uri:    "rtsp://127.0.0.1/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102",
			headers: map[string]string{
				"transport": "RTP/AVP;unicast;destination=127.0.0.1;client_port=" + strconv.Itoa(rtpPort) + "-" + strconv.Itoa(rtpPort+1),
			},
		},
		"1",
	)
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}
	match := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)
	if len(match) < 2 {
		t.Fatalf("missing session id: %s", setup)
	}
	sessionID := match[1]

	play := server.handlePlay(request{headers: map[string]string{"session": sessionID}}, "2")
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}
	defer server.handleTeardown(request{headers: map[string]string{"session": sessionID}}, "3")

	buf := make([]byte, 2048)
	for i := 0; i < 3; i++ {
		_ = rtpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		if _, _, err := rtpConn.ReadFromUDP(buf); err != nil {
			t.Fatalf("packet %d: %v", i+1, err)
		}
	}
	_ = rtpConn.SetReadDeadline(time.Now().Add(80 * time.Millisecond))
	if _, _, err := rtpConn.ReadFromUDP(buf); err == nil {
		t.Fatal("expected rtp_stop scenario to stop after burst")
	}
}

func TestPlayUsesMalformedPSIScenarioPayload(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioMalformedPSI); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()
	rtpPort := rtpConn.LocalAddr().(*net.UDPAddr).Port

	setup := server.handleSetup(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{
			method: "SETUP",
			uri:    "rtsp://127.0.0.1/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102",
			headers: map[string]string{
				"transport": "RTP/AVP;unicast;destination=127.0.0.1;client_port=" + strconv.Itoa(rtpPort) + "-" + strconv.Itoa(rtpPort+1),
			},
		},
		"1",
	)
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}
	sessionID := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)[1]

	play := server.handlePlay(request{headers: map[string]string{"session": sessionID}}, "2")
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}
	defer server.handleTeardown(request{headers: map[string]string{"session": sessionID}}, "3")

	buf := make([]byte, 2048)
	_ = rtpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n, _, err := rtpConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < 12+188 || buf[12] != 0x47 {
		t.Fatalf("unexpected RTP/TS packet: n=%d sync=0x%x", n, buf[12])
	}
	if got := buf[12+5]; got != 0xFF {
		t.Fatalf("expected malformed PAT table id, got 0x%x", got)
	}
}

func TestPlayUsesContinuityErrorScenarioPayload(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioContinuityErrors); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	payload, err := server.playPayload(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}, lab.Service{ID: "das-erste-hd"}, lab.Mux{ID: "src1-11494h-22000-dvbs2"})
	if err != nil {
		t.Fatal(err)
	}

	if payload[0] != 0x47 {
		t.Fatalf("sync byte changed: 0x%x", payload[0])
	}
	if got := payload[3] & 0x0F; got == 0 {
		t.Fatal("expected continuity counter to be corrupted")
	}
}

func TestPlayPayloadSkipsScenarioWhenTargetDoesNotMatch(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioContinuityErrors, "zdf-hd", ""); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{PublicHost: "127.0.0.1"}, &ts.Source{}, manager)
	profile := ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}
	payload, err := server.playPayload(profile, lab.Service{ID: "das-erste-hd"}, lab.Mux{ID: "src1-11494h-22000-dvbs2"})
	if err != nil {
		t.Fatal(err)
	}

	if got := payload[3] & 0x0F; got != 0 {
		t.Fatalf("non-targeted service should keep continuity counter, got %d", got)
	}
}

func TestPlayPayloadUsesConfiguredEPGClockForEIT(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	server := NewServer(config.Config{
		PublicHost: "127.0.0.1",
		EPGClock:   "fixed:2026-03-29T04:45:00+02:00",
	}, &ts.Source{}, manager)
	profile := ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}
	payload, err := server.playPayload(profile, lab.Service{ID: "das-erste-hd"}, lab.Mux{ID: "src1-11494h-22000-dvbs2"})
	if err != nil {
		t.Fatal(err)
	}
	section := firstSectionByPID(payload, 0x12)
	if len(section) == 0 {
		t.Fatal("missing EIT section")
	}
	if got := section[18:21]; !bytes.Equal(got, []byte{0x02, 0x45, 0x00}) {
		t.Fatalf("EIT UTC start from configured EPG clock: got % x", got)
	}
}

func TestPlayPayloadSuppressesEITForTargetedEPGGap(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioEPGGap, "das-erste-hd", ""); err != nil {
		t.Fatal(err)
	}
	server := NewServer(config.Config{
		PublicHost: "127.0.0.1",
		EPGClock:   "fixed:2026-03-29T01:30:00+01:00",
	}, &ts.Source{}, manager)
	profile := ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}
	payload, err := server.playPayload(profile, lab.Service{ID: "das-erste-hd"}, lab.Mux{ID: "src1-11494h-22000-dvbs2"})
	if err != nil {
		t.Fatal(err)
	}
	if section := firstSectionByPID(payload, 0x12); len(section) != 0 {
		t.Fatalf("targeted epg_gap should suppress EIT, got section % x", section)
	}
	if !strings.Contains(string(payload), "das-erste-hd") {
		t.Fatal("synthetic payload should still contain service media markers")
	}
}

func TestStreamBehaviorDropsEveryNthPacket(t *testing.T) {
	behavior := streamBehavior{dropEvery: 3}

	if behavior.shouldDrop(1) || behavior.shouldDrop(2) {
		t.Fatal("first two packets should not be dropped")
	}
	if !behavior.shouldDrop(3) {
		t.Fatal("third packet should be dropped")
	}
	if behavior.shouldDrop(4) || behavior.shouldDrop(5) {
		t.Fatal("packet drops should resume after the third packet")
	}
	if !behavior.shouldDrop(6) {
		t.Fatal("sixth packet should be dropped")
	}
}

func firstSectionByPID(payload []byte, pid uint16) []byte {
	for offset := 0; offset+188 <= len(payload); offset += 188 {
		packet := payload[offset : offset+188]
		if ts.PID(packet) != pid || packet[1]&0x40 == 0 {
			continue
		}
		start := 5 + int(packet[4])
		if start+3 > len(packet) {
			continue
		}
		length := int(packet[start+1]&0x0F)<<8 | int(packet[start+2])
		end := start + 3 + length
		if end > len(packet) {
			continue
		}
		return append([]byte(nil), packet[start:end]...)
	}
	return nil
}

func TestStreamBehaviorAppliesJitterEveryNthPacket(t *testing.T) {
	behavior := streamBehavior{jitterEvery: 3, jitterDelay: 40 * time.Millisecond}

	if got := behavior.jitterFor(1); got != 0 {
		t.Fatalf("packet 1 jitter: got %s", got)
	}
	if got := behavior.jitterFor(3); got != 40*time.Millisecond {
		t.Fatalf("packet 3 jitter: got %s", got)
	}
	if got := behavior.jitterFor(6); got != 40*time.Millisecond {
		t.Fatalf("packet 6 jitter: got %s", got)
	}
}

type fakeTCPConn struct {
	remote net.Addr
}

func setupTestSession(t *testing.T, server *Server) string {
	t.Helper()
	return setupTestSessionWithRTPPort(t, server, 6000)
}

func setupTestSessionWithRTPPort(t *testing.T, server *Server, rtpPort int) string {
	t.Helper()
	setup := server.handleSetup(
		&fakeTCPConn{remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 55000}},
		request{
			method: "SETUP",
			uri:    "rtsp://127.0.0.1/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102",
			headers: map[string]string{
				"transport": "RTP/AVP;unicast;destination=127.0.0.1;client_port=" + strconv.Itoa(rtpPort) + "-" + strconv.Itoa(rtpPort+1),
			},
		},
		"1",
	)
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}
	match := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)
	if len(match) < 2 {
		t.Fatalf("missing session id: %s", setup)
	}
	return match[1]
}

func sameInts(got, want []int) bool {
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

func (f *fakeTCPConn) Read(_ []byte) (int, error)         { return 0, nil }
func (f *fakeTCPConn) Write(b []byte) (int, error)        { return len(b), nil }
func (f *fakeTCPConn) Close() error                       { return nil }
func (f *fakeTCPConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (f *fakeTCPConn) RemoteAddr() net.Addr               { return f.remote }
func (f *fakeTCPConn) SetDeadline(_ time.Time) error      { return nil }
func (f *fakeTCPConn) SetReadDeadline(_ time.Time) error  { return nil }
func (f *fakeTCPConn) SetWriteDeadline(_ time.Time) error { return nil }
