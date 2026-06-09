package topology_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/topology"
)

func TestLoadFileAllowsDuplicateFriendlyNamesAndStaleLocation(t *testing.T) {
	path := writeTopology(t, `
devices:
  - id: lab-a
    friendly_name: SATIP Twin
    profile: generic-satip-1.2
    public_host: 127.0.0.1
    http_port: 18875
    rtsp_port: 1554
    tuners: 2
  - id: lab-b
    friendly_name: SATIP Twin
    profile: tvheadend
    public_host: 127.0.0.1
    http_port: 18876
    rtsp_port: 1555
    tuners: 4
    location: http://192.0.2.10:8875/stale-desc.xml
    stale_location: true
`)

	doc, err := topology.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}
	if len(doc.Devices) != 2 {
		t.Fatalf("devices: got %d", len(doc.Devices))
	}
	if doc.Devices[0].Location != "http://127.0.0.1:18875/desc.xml" {
		t.Fatalf("device A location: got %q", doc.Devices[0].Location)
	}
	if doc.Devices[1].FriendlyName != doc.Devices[0].FriendlyName {
		t.Fatalf("duplicate friendly names should be preserved: %+v", doc.Devices)
	}
	if !doc.Devices[1].StaleLocation || doc.Devices[1].Location != "http://192.0.2.10:8875/stale-desc.xml" {
		t.Fatalf("stale location not preserved: %+v", doc.Devices[1])
	}
	if doc.Devices[1].DescriptionPath != "/satip_server/desc.xml" {
		t.Fatalf("profile-specific description path: got %q", doc.Devices[1].DescriptionPath)
	}
}

func TestLoadFileRejectsDuplicateIDs(t *testing.T) {
	path := writeTopology(t, `
devices:
  - id: lab-a
    friendly_name: A
    profile: spec
    public_host: 127.0.0.1
    http_port: 18875
    rtsp_port: 1554
    tuners: 1
  - id: lab-a
    friendly_name: B
    profile: spec
    public_host: 127.0.0.1
    http_port: 18876
    rtsp_port: 1555
    tuners: 1
`)

	_, err := topology.LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate device id") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestLoadFileRejectsUnknownProfile(t *testing.T) {
	path := writeTopology(t, `
devices:
  - id: lab-a
    friendly_name: A
    profile: tvhedend
    public_host: 127.0.0.1
    http_port: 18875
    rtsp_port: 1554
    tuners: 1
`)

	_, err := topology.LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "devices[0].profile") || !strings.Contains(err.Error(), "tvhedend") {
		t.Fatalf("expected unknown profile field error, got %v", err)
	}
}

func writeTopology(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "topology.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
