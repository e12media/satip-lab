package httpserver_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/httpserver"
	"github.com/e12media/satip-lab/internal/lab"
)

func TestStartReturnsPortBindError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	server := httpserver.New(config.Config{
		BindAddress: "127.0.0.1",
		HTTPPort:    port,
	}, nil)

	if err := server.Start(); err == nil {
		t.Fatal("expected start to report port bind error")
	}
}

func TestStatusPageShowsRuntimeScenario(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	if err := labManager.SetScenario(lab.ScenarioNoSignal); err != nil {
		t.Fatal(err)
	}
	server := httpserver.New(config.Config{Scenario: config.ScenarioNormal}, labManager)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Runtime scenario: no_signal") {
		t.Fatalf("status page should show runtime scenario: %s", rec.Body.String())
	}
}

func TestProfileSpecificM3UPathServesPlaylist(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{
		PublicHost: "127.0.0.1",
		RTSPPort:   554,
		Profile:    "tvheadend",
	}, labManager)
	req := httptest.NewRequest(http.MethodGet, "/channellist.m3u", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "#EXTM3U") {
		t.Fatalf("missing playlist body: %s", rec.Body.String())
	}
}

func TestProfileSpecificDeviceDescriptionPathServesXML(t *testing.T) {
	labManager := lab.NewManager(lab.DefaultCatalog(), 2)
	server := httpserver.New(config.Config{
		Profile: "digital-devices-octopus-net",
	}, labManager)
	req := httptest.NewRequest(http.MethodGet, "/octoserve/octonet.xml", nil)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code: got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<modelName>Octopus NET</modelName>") {
		t.Fatalf("missing profile XML body: %s", rec.Body.String())
	}
}
