package simulator_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/simulator"
)

func TestHTTPServesDescriptionAndM3U(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	resp, err := http.Get(cfg.DeviceDescriptionURL())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), config.SatIPSearchTarget) {
		t.Fatal("missing device type in description")
	}

	m3uResp, err := http.Get(cfg.M3UURL())
	if err != nil {
		t.Fatal(err)
	}
	defer m3uResp.Body.Close()
	m3u, _ := io.ReadAll(m3uResp.Body)
	text := string(m3u)
	if !strings.Contains(text, "#EXTM3U") || !strings.Contains(text, "ZDF HD") {
		t.Fatal("unexpected m3u content")
	}
}

func TestConfiguredCatalogDrivesM3UAndAPI(t *testing.T) {
	catalogPath := writeSimulatorCatalogFixture(t, `services:
  - id: custom-news-hd
    number: 201
    name: Custom News HD
    group: Lab
    tvg_id: custom-news.example
    src: 1
    freq: 12188
    pol: h
    sr: 27500
    msys: dvbs2
    pids: [0, 17, 8100, 8101, 8102]
`)
	cfg, sim := startTestSimulatorWithConfig(t, func(cfg *config.Config) {
		cfg.CatalogPath = catalogPath
	})
	defer stopTestSimulator(sim)

	m3uResp, err := http.Get(cfg.M3UURL())
	if err != nil {
		t.Fatal(err)
	}
	defer m3uResp.Body.Close()
	m3u, _ := io.ReadAll(m3uResp.Body)
	text := string(m3u)
	if !strings.Contains(text, "Custom News HD") || strings.Contains(text, "ZDF HD") {
		t.Fatalf("unexpected m3u content:\n%s", text)
	}
	if !strings.Contains(text, "freq=12188") || !strings.Contains(text, "pids=0,17,8100,8101,8102") {
		t.Fatalf("missing custom tuning parameters:\n%s", text)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/catalog", cfg.HTTPPort))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Services []struct {
			ID    string `json:"id"`
			TvgID string `json:"tvg_id"`
		} `json:"services"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Services) != 1 || body.Services[0].ID != "custom-news-hd" || body.Services[0].TvgID != "custom-news.example" {
		t.Fatalf("unexpected catalog API: %+v", body)
	}
}

func TestSimulatorRejectsInvalidCatalogBeforeStart(t *testing.T) {
	catalogPath := writeSimulatorCatalogFixture(t, `services:
  - id: invalid
    number: 1
    name: Invalid
    group: Lab
    tvg_id: invalid.example
    src: 1
    freq: 12188
    pol: bad
    sr: 27500
    msys: dvbs2
    pids: [0, 17, 8100, 8101, 8102]
`)
	_, err := simulator.New(config.Config{CatalogPath: catalogPath})
	if err == nil {
		t.Fatal("expected catalog validation error")
	}
	if !strings.Contains(err.Error(), "catalog") || !strings.Contains(err.Error(), "pol") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSimulatorRejectsInvalidTopologyBeforeStart(t *testing.T) {
	topologyPath := writeSimulatorCatalogFixture(t, `devices:
  - id: lab-a
    friendly_name: SATIP Twin
    profile: spec
    public_host: 127.0.0.1
    http_port: 18875
    rtsp_port: 1554
    tuners: 2
  - id: lab-a
    friendly_name: Duplicate ID
    profile: spec
    public_host: 127.0.0.1
    http_port: 18876
    rtsp_port: 1555
    tuners: 2
`)
	_, err := simulator.New(config.Config{TopologyPath: topologyPath})
	if err == nil {
		t.Fatal("expected topology validation error")
	}
	if !strings.Contains(err.Error(), "duplicate device id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRTSPSessionOptionsSetupPlayTeardown(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	options := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"OPTIONS rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 1\r\n\r\n", cfg.RTSPPort,
	))
	if !strings.Contains(options, "200 OK") {
		t.Fatalf("OPTIONS failed: %s", options)
	}

	setup := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 2\r\nTransport: RTP/AVP;unicast;client_port=5000-5001\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(setup, "200 OK") || !strings.Contains(setup, "Session:") {
		t.Fatalf("SETUP failed: %s", setup)
	}

	re := regexp.MustCompile(`Session: (\d+)`)
	match := re.FindStringSubmatch(setup)
	if len(match) < 2 {
		t.Fatal("no session id")
	}
	sessionID := match[1]

	play := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"PLAY rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}

	teardown := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"TEARDOWN rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 4\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(teardown, "200 OK") {
		t.Fatalf("TEARDOWN failed: %s", teardown)
	}
}

func TestRTSPPlaySendsRTPMPEGTSPackets(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()
	rtpPort := rtpConn.LocalAddr().(*net.UDPAddr).Port

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.RTSPPort), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	setup := rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: RTP/AVP;unicast;destination=127.0.0.1;client_port=%d-%d\r\n\r\n",
		cfg.RTSPPort, rtpPort, rtpPort+1,
	))
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}

	re := regexp.MustCompile(`Session: (\d+)`)
	match := re.FindStringSubmatch(setup)
	if len(match) < 2 {
		t.Fatal("no session id")
	}
	sessionID := match[1]

	play := rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"PLAY rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 2\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(play, "200 OK") || !strings.Contains(play, "Session: "+sessionID) {
		t.Fatalf("PLAY failed: %s", play)
	}

	buf := make([]byte, 2048)
	_ = rtpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := rtpConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < 13 {
		t.Fatalf("RTP packet too small: %d", n)
	}
	if got := buf[0] >> 6; got != 2 {
		t.Fatalf("RTP version: got %d", got)
	}
	if got := buf[1] & 0x7F; got != 33 {
		t.Fatalf("RTP payload type: got %d", got)
	}
	if got := buf[12]; got != 0x47 {
		t.Fatalf("MPEG-TS sync byte: got 0x%x", got)
	}

	teardown := rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"TEARDOWN rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(teardown, "200 OK") || !strings.Contains(teardown, "Session: "+sessionID) {
		t.Fatalf("TEARDOWN failed: %s", teardown)
	}
}

func TestGeneratedRTPPayloadDiffersByService(t *testing.T) {
	cfg, sim := startGeneratedStreamSimulator(t)
	defer stopTestSimulator(sim)

	dasErste := playAndReadRTPPayload(t, cfg.RTSPPort, "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", 56100)
	zdf := playAndReadRTPPayload(t, cfg.RTSPPort, "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", 56110)

	if string(dasErste) == string(zdf) {
		t.Fatal("expected distinct RTP TS payloads for different services")
	}
	if !strings.Contains(string(dasErste), "das-erste-hd") {
		t.Fatalf("missing Das Erste marker in payload: %q", string(dasErste))
	}
	if !strings.Contains(string(zdf), "zdf-hd") {
		t.Fatalf("missing ZDF marker in payload: %q", string(zdf))
	}
}

func TestLabAPIReportsSessionsTunersAndEvents(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	setup := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: RTP/AVP;unicast;client_port=5000-5001\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}

	status := fetchStatus(t, cfg.HTTPPort)
	if len(status.Sessions) != 1 {
		t.Fatalf("sessions: got %+v", status.Sessions)
	}
	if status.Sessions[0].ServiceID != "das-erste-hd" || status.Tuners[0].State != "tuned" {
		t.Fatalf("unexpected lab status: %+v", status)
	}
	if len(status.Events) == 0 {
		t.Fatal("expected lab events")
	}
}

func TestAPIResetClearsRTSPSessions(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	setup := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: RTP/AVP;unicast;client_port=5000-5001\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}
	sessionID := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)[1]

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/reset", cfg.HTTPPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset status: got %d", resp.StatusCode)
	}

	play := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"PLAY rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 2\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(play, "454 Session Not Found") {
		t.Fatalf("expected reset to clear RTSP session, got: %s", play)
	}
}

func TestSingleTunerRejectsDifferentMuxUntilTeardown(t *testing.T) {
	httpPort := freeTCPPort(t)
	rtspPort := freeTCPPort(t)
	cfg := config.Config{
		BindAddress:         "0.0.0.0",
		PublicHost:          "127.0.0.1",
		HTTPPort:            httpPort,
		RTSPPort:            rtspPort,
		SSDPort:             0,
		TunerCount:          1,
		TransportStreamPath: "assets/sample.ts",
		Scenario:            config.ScenarioNormal,
	}
	sim, err := simulator.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Start(); err != nil {
		t.Fatal(err)
	}
	defer stopTestSimulator(sim)
	time.Sleep(50 * time.Millisecond)

	setup := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: RTP/AVP;unicast;client_port=5000-5001\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("first SETUP failed: %s", setup)
	}
	sessionID := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)[1]

	busy := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120 RTSP/1.0\r\n"+
			"CSeq: 2\r\nTransport: RTP/AVP;unicast;client_port=5002-5003\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(busy, "503 Service Unavailable") {
		t.Fatalf("expected tuner busy: %s", busy)
	}

	teardown := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"TEARDOWN rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		cfg.RTSPPort, sessionID,
	))
	if !strings.Contains(teardown, "200 OK") {
		t.Fatalf("TEARDOWN failed: %s", teardown)
	}

	afterRelease := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120 RTSP/1.0\r\n"+
			"CSeq: 4\r\nTransport: RTP/AVP;unicast;client_port=5004-5005\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(afterRelease, "200 OK") {
		t.Fatalf("expected setup after release: %s", afterRelease)
	}
}

func TestRuntimeTunerBusyScenarioRejectsSetupWithoutAllocatingState(t *testing.T) {
	cfg, sim := startTestSimulator(t)
	defer stopTestSimulator(sim)

	body := bytes.NewBufferString(`{"name":"tuner_busy"}`)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/api/scenario", cfg.HTTPPort), body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scenario status: got %d", resp.StatusCode)
	}

	setup := rtspExchange(t, cfg.RTSPPort, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: RTP/AVP;unicast;client_port=5000-5001\r\n\r\n",
		cfg.RTSPPort,
	))
	if !strings.Contains(setup, "503 Service Unavailable") || !strings.Contains(setup, "Reason: tuner busy") {
		t.Fatalf("expected runtime tuner_busy 503: %s", setup)
	}

	status := fetchStatus(t, cfg.HTTPPort)
	if len(status.Sessions) != 0 {
		t.Fatalf("tuner_busy should not allocate sessions: %+v", status.Sessions)
	}
	for _, tuner := range status.Tuners {
		if tuner.State != "idle" {
			t.Fatalf("tuner_busy should not allocate tuners: %+v", status.Tuners)
		}
	}
}

func startTestSimulator(t *testing.T) (config.Config, *simulator.Simulator) {
	t.Helper()
	return startTestSimulatorWithConfig(t, nil)
}

func startTestSimulatorWithConfig(t *testing.T, mutate func(*config.Config)) (config.Config, *simulator.Simulator) {
	t.Helper()
	httpPort := freeTCPPort(t)
	rtspPort := freeTCPPort(t)
	cfg := config.Config{
		BindAddress:         "0.0.0.0",
		PublicHost:          "127.0.0.1",
		HTTPPort:            httpPort,
		RTSPPort:            rtspPort,
		SSDPort:             0,
		TunerCount:          2,
		TransportStreamPath: "assets/sample.ts",
		Scenario:            config.ScenarioNormal,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	sim, err := simulator.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	return cfg, sim
}

func startGeneratedStreamSimulator(t *testing.T) (config.Config, *simulator.Simulator) {
	t.Helper()
	httpPort := freeTCPPort(t)
	rtspPort := freeTCPPort(t)
	cfg := config.Config{
		BindAddress: "0.0.0.0",
		PublicHost:  "127.0.0.1",
		HTTPPort:    httpPort,
		RTSPPort:    rtspPort,
		SSDPort:     0,
		TunerCount:  2,
		Scenario:    config.ScenarioNormal,
	}
	sim, err := simulator.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	return cfg, sim
}

func writeSimulatorCatalogFixture(t *testing.T, body string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "catalog-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

type labStatus struct {
	Tuners []struct {
		ID    int    `json:"id"`
		State string `json:"state"`
		MuxID string `json:"mux_id"`
	} `json:"tuners"`
	Sessions []struct {
		ID        string `json:"id"`
		ServiceID string `json:"service_id"`
		State     string `json:"state"`
	} `json:"sessions"`
	Events []struct {
		Type string `json:"type"`
	} `json:"events"`
}

func fetchStatus(t *testing.T, port int) labStatus {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var status labStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func stopTestSimulator(sim *simulator.Simulator) {
	_ = sim.Stop()
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func rtspExchange(t *testing.T, port int, request string) string {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte(request))
	if err != nil {
		t.Fatal(err)
	}

	return readRTSPResponse(t, conn)
}

func rtspExchangeOnConn(t *testing.T, conn net.Conn, request string) string {
	t.Helper()
	_, err := conn.Write([]byte(request))
	if err != nil {
		t.Fatal(err)
	}

	return readRTSPResponse(t, conn)
}

func playAndReadRTPPayload(t *testing.T, rtspPort int, tuningQuery string, rtpPort int) []byte {
	t.Helper()
	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: rtpPort})
	if err != nil {
		t.Fatal(err)
	}
	defer rtpConn.Close()

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", rtspPort), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	setup := rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"SETUP rtsp://127.0.0.1:%d/?%s RTSP/1.0\r\nCSeq: 1\r\nTransport: RTP/AVP;unicast;destination=127.0.0.1;client_port=%d-%d\r\n\r\n",
		rtspPort, tuningQuery, rtpPort, rtpPort+1,
	))
	if !strings.Contains(setup, "200 OK") {
		t.Fatalf("SETUP failed: %s", setup)
	}
	sessionID := regexp.MustCompile(`Session: (\d+)`).FindStringSubmatch(setup)[1]
	play := rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"PLAY rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 2\r\nSession: %s\r\n\r\n",
		rtspPort, sessionID,
	))
	if !strings.Contains(play, "200 OK") {
		t.Fatalf("PLAY failed: %s", play)
	}
	buf := make([]byte, 2048)
	_ = rtpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := rtpConn.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	_ = rtspExchangeOnConn(t, conn, fmt.Sprintf(
		"TEARDOWN rtsp://127.0.0.1:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		rtspPort, sessionID,
	))
	if n <= 12 {
		t.Fatalf("RTP packet too short: %d", n)
	}
	return append([]byte(nil), buf[12:n]...)
}

func readRTSPResponse(t *testing.T, conn net.Conn) string {
	t.Helper()
	reader := bufio.NewReader(conn)
	var buf strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString(line)
		if strings.HasSuffix(buf.String(), "\r\n\r\n") {
			return buf.String()
		}
	}
}
