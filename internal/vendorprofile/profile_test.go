package vendorprofile_test

import (
	"reflect"
	"testing"

	"github.com/e12media/satip-lab/internal/vendorprofile"
)

func TestSpecProfileDefinesStrictSATIPBehavior(t *testing.T) {
	profile := vendorprofile.ForName(vendorprofile.NameSpec)

	if profile.Name != vendorprofile.NameSpec {
		t.Fatalf("name: got %q", profile.Name)
	}
	if profile.SessionHeader != "Session" {
		t.Fatalf("session header: got %q", profile.SessionHeader)
	}
	if profile.TransportHeader != "Transport" {
		t.Fatalf("transport header: got %q", profile.TransportHeader)
	}
	if profile.SessionIDFormat != vendorprofile.SessionIDNumeric {
		t.Fatalf("session id format: got %q", profile.SessionIDFormat)
	}
	if !profile.IncludeSetupTimeout {
		t.Fatal("spec profile should include timeout on SETUP Session header")
	}
	if profile.RequireDescribeBeforeSetup {
		t.Fatal("spec profile must allow SETUP without DESCRIBE")
	}
	if profile.TunerBusyStatus != "503 Service Unavailable" {
		t.Fatalf("tuner busy status: got %q", profile.TunerBusyStatus)
	}
	if profile.Device.FriendlyName != "satip-lab" {
		t.Fatalf("friendly name: got %q", profile.Device.FriendlyName)
	}
	if profile.Device.XSatipM3U != "/channels.m3u" {
		t.Fatalf("m3u path: got %q", profile.Device.XSatipM3U)
	}
	if profile.SSDP.Server != "satip-lab/0.2 UPnP/1.0" {
		t.Fatalf("ssdp server: got %q", profile.SSDP.Server)
	}
}

func TestUnknownProfileFallsBackToSpec(t *testing.T) {
	if got := vendorprofile.ForName("triax").Name; got != vendorprofile.NameSpec {
		t.Fatalf("unknown profile should fall back to spec, got %q", got)
	}
}

func TestNamesListsImplementedProfiles(t *testing.T) {
	want := []string{
		vendorprofile.NameGeneric,
		vendorprofile.NameSpec,
		"minisatip",
		"tvheadend",
		"triax-tss400",
		"telestar-digibit-r1",
		"kathrein-exip",
		"digital-devices-octopus-net",
	}
	if got := vendorprofile.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("profile names: got %#v", got)
	}
}

func TestNamedCompatibilityProfilesExposeDeviceAndEvidenceMetadata(t *testing.T) {
	profile := vendorprofile.ForName("telestar-digibit-r1")

	if profile.Name != "telestar-digibit-r1" {
		t.Fatalf("name: got %q", profile.Name)
	}
	if profile.Device.Manufacturer == "" || profile.Device.ModelName == "" {
		t.Fatalf("device metadata should be populated: %#v", profile.Device)
	}
	if profile.Evidence.Confidence == "" || len(profile.Evidence.Sources) == 0 {
		t.Fatalf("evidence metadata should be populated: %#v", profile.Evidence)
	}
	if profile.SessionHeader != "Session" {
		t.Fatalf("non-trace-backed profile should keep spec RTSP behavior, got %q", profile.SessionHeader)
	}
}
