package smoke_test

import (
	"context"
	"fmt"
	"net"
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
