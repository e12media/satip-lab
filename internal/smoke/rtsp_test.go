package smoke_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/simulator"
	"github.com/e12media/satip-lab/internal/smoke"
)

func TestProbeRTSPRTPReceivesMPEGTS(t *testing.T) {
	cfg, sim := startSimulator(t)
	defer func() { _ = sim.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := smoke.ProbeRTSPRTP(ctx, smoke.RTSPProbeOptions{
		Host: cfg.PublicHost,
		Port: cfg.RTSPPort,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PayloadType != 33 {
		t.Fatalf("payload type: got %d", result.PayloadType)
	}
	if result.MPEGTSSyncByte != 0x47 {
		t.Fatalf("MPEG-TS sync byte: got 0x%x", result.MPEGTSSyncByte)
	}
}

func TestProbeRTSPRTPRecordsJSONEvidence(t *testing.T) {
	cfg, sim := startSimulator(t)
	defer func() { _ = sim.Stop() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := smoke.ProbeRTSPRTP(ctx, smoke.RTSPProbeOptions{
		Host:    cfg.PublicHost,
		Port:    cfg.RTSPPort,
		Profile: "tvheadend",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := result.JSONEvidence()
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Profile         string `json:"profile"`
		SessionID       string `json:"session_id"`
		SessionIDFormat string `json:"session_id_format"`
		RTP             struct {
			Bytes          int    `json:"bytes"`
			PayloadType    byte   `json:"payload_type"`
			MPEGTSSyncByte string `json:"mpeg_ts_sync_byte"`
		} `json:"rtp"`
		RTSP []struct {
			Method     string            `json:"method"`
			StatusLine string            `json:"status_line"`
			Headers    map[string]string `json:"headers"`
			DurationMS int64             `json:"duration_ms"`
		} `json:"rtsp"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Profile != "tvheadend" {
		t.Fatalf("profile: got %q", doc.Profile)
	}
	if doc.SessionID == "" || doc.SessionIDFormat != "numeric" {
		t.Fatalf("session evidence: id=%q format=%q", doc.SessionID, doc.SessionIDFormat)
	}
	if doc.RTP.PayloadType != 33 || doc.RTP.Bytes == 0 || doc.RTP.MPEGTSSyncByte != "0x47" {
		t.Fatalf("rtp evidence: %#v", doc.RTP)
	}
	if len(doc.RTSP) != 3 {
		t.Fatalf("rtsp exchange count: got %d", len(doc.RTSP))
	}
	if doc.RTSP[0].Method != "SETUP" || !strings.Contains(doc.RTSP[0].StatusLine, "200 OK") {
		t.Fatalf("setup evidence: %#v", doc.RTSP[0])
	}
	if doc.RTSP[0].Headers["Session"] == "" || doc.RTSP[0].Headers["Transport"] == "" {
		t.Fatalf("setup headers: %#v", doc.RTSP[0].Headers)
	}
	if !strings.Contains(doc.RTSP[0].Headers["Session"], doc.SessionID) || !strings.Contains(doc.RTSP[0].Headers["Session"], "timeout=") {
		t.Fatalf("setup Session header should preserve observed parameters: %#v", doc.RTSP[0].Headers["Session"])
	}
	if doc.RTSP[1].Method != "PLAY" || !strings.Contains(doc.RTSP[1].StatusLine, "200 OK") {
		t.Fatalf("play evidence: %#v", doc.RTSP[1])
	}
	if doc.RTSP[2].Method != "TEARDOWN" || !strings.Contains(doc.RTSP[2].StatusLine, "200 OK") {
		t.Fatalf("teardown evidence: %#v", doc.RTSP[2])
	}
	for _, exchange := range doc.RTSP {
		if exchange.DurationMS < 0 {
			t.Fatalf("duration should be non-negative: %#v", exchange)
		}
	}
}

func startSimulator(t *testing.T) (config.Config, *simulator.Simulator) {
	t.Helper()
	cfg := config.Config{
		BindAddress:         "127.0.0.1",
		PublicHost:          "127.0.0.1",
		HTTPPort:            freeTCPPort(t),
		RTSPPort:            freeTCPPort(t),
		SSDPort:             0,
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
	time.Sleep(50 * time.Millisecond)
	return cfg, sim
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func ExampleProbeRTSPRTP() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _ = smoke.ProbeRTSPRTP(ctx, smoke.RTSPProbeOptions{
		Host: "127.0.0.1",
		Port: 554,
	})
	fmt.Println("probe attempted")
	// Output: probe attempted
}
