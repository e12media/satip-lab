package ssdp

import (
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/config"
)

func TestSearchResponseUsesCompatibilityProfileHeaders(t *testing.T) {
	server := New(config.Config{
		PublicHost: "192.0.2.44",
		HTTPPort:   8875,
		Profile:    "minisatip",
	})

	response := server.searchResponse()

	if !strings.Contains(response, "SERVER: minisatip") {
		t.Fatalf("response should use profile server header:\n%s", response)
	}
	if !strings.Contains(response, "USN: uuid:minisatip-lab::urn:ses-com:device:SatIPServer:1") {
		t.Fatalf("response should use profile USN:\n%s", response)
	}
}
