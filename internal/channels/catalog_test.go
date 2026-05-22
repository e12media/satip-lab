package channels_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/channels"
	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/vendorprofile"
)

func TestBuildM3UIncludesFiveDachChannels(t *testing.T) {
	m3u := channels.BuildM3U("192.168.1.50", 554, nil)

	if !strings.HasPrefix(m3u, "#EXTM3U") {
		t.Fatal("expected EXTM3U header")
	}
	for _, name := range []string{"Das Erste HD", "ZDF HD", "arte HD", "3sat HD", "phoenix HD"} {
		if !strings.Contains(m3u, name) {
			t.Fatalf("missing channel %s", name)
		}
	}
	if !strings.Contains(m3u, "rtsp://192.168.1.50:554/?") {
		t.Fatal("expected rtsp urls")
	}
	if !strings.Contains(m3u, "freq=11494") || !strings.Contains(m3u, "msys=dvbs2") {
		t.Fatal("expected tuning parameters")
	}
}

func TestBuildDeviceDescriptionXML(t *testing.T) {
	body := channels.BuildDeviceDescriptionXML("/", 4, vendorprofile.ForName(vendorprofile.NameSpec))
	var root struct {
		XMLName     xml.Name `xml:"root"`
		ConfigID    string   `xml:"configId,attr"`
		SpecVersion struct {
			Major int `xml:"major"`
			Minor int `xml:"minor"`
		} `xml:"specVersion"`
		Device struct {
			DeviceType      string `xml:"deviceType"`
			FriendlyName    string `xml:"friendlyName"`
			PresentationURL string `xml:"presentationURL"`
			SATIPCap        string `xml:"urn:ses-com:satip X_SATIPCAP"`
			SATIPM3U        string `xml:"urn:ses-com:satip X_SATIPM3U"`
		} `xml:"device"`
	}
	if err := xml.Unmarshal([]byte(body), &root); err != nil {
		t.Fatal(err)
	}
	if root.ConfigID != "1" || root.SpecVersion.Major != 1 || root.SpecVersion.Minor != 0 {
		t.Fatalf("unexpected UPnP root metadata: %+v", root)
	}
	if root.Device.DeviceType != config.SatIPSearchTarget {
		t.Fatal("missing device type")
	}
	if root.Device.FriendlyName != "satip-lab" {
		t.Fatal("missing friendly name")
	}
	if root.Device.PresentationURL != "/" {
		t.Fatalf("unexpected presentation URL: %q", root.Device.PresentationURL)
	}
	if root.Device.SATIPCap != "DVBS2-4" || root.Device.SATIPM3U != "/channels.m3u" {
		t.Fatalf("unexpected SAT>IP extensions: %+v", root.Device)
	}
}

func TestBuildDeviceDescriptionXMLUsesCompatibilityProfile(t *testing.T) {
	profile := vendorprofile.ForName("tvheadend")
	body := channels.BuildDeviceDescriptionXML("/", 4, profile)

	if !strings.Contains(body, "<friendlyName>TVHeadend SAT&gt;IP</friendlyName>") {
		t.Fatalf("description should use profile friendly name:\n%s", body)
	}
	if !strings.Contains(body, "<manufacturer>TVHeadend</manufacturer>") {
		t.Fatalf("description should use profile manufacturer:\n%s", body)
	}
	if !strings.Contains(body, "<satip:X_SATIPM3U>/channellist.m3u</satip:X_SATIPM3U>") {
		t.Fatalf("description should use profile M3U path:\n%s", body)
	}
}
