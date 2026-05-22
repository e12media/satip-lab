package channels

import (
	"fmt"
	"html"
	"strings"

	"github.com/e12media/satip-lab/internal/vendorprofile"
)

var DachChannels = []Channel{
	{ID: "das-erste-hd", Number: 1, Name: "Das Erste HD", Group: "DE", TvgID: "daserste.de", Frequency: 11494, Polarization: "h", SymbolRate: 22000, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 5100, 5101, 5102}},
	{ID: "zdf-hd", Number: 2, Name: "ZDF HD", Group: "DE", TvgID: "zdf.de", Frequency: 11362, Polarization: "h", SymbolRate: 22000, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 6100, 6110, 6120}},
	{ID: "arte-hd", Number: 3, Name: "arte HD", Group: "DE", TvgID: "arte.de", Frequency: 11494, Polarization: "h", SymbolRate: 22000, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 5200, 5201, 5202}},
	{ID: "3sat-hd", Number: 4, Name: "3sat HD", Group: "DE", TvgID: "3sat.de", Frequency: 11347, Polarization: "v", SymbolRate: 22000, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 6300, 6310, 6320}},
	{ID: "phoenix-hd", Number: 5, Name: "phoenix HD", Group: "DE", TvgID: "phoenix.de", Frequency: 11582, Polarization: "h", SymbolRate: 22000, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 6500, 6510, 6520}},
}

func BuildM3U(publicHost string, publicRTSPPort int, list []Channel) string {
	if list == nil {
		list = DachChannels
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	for _, ch := range list {
		fmt.Fprintf(&b,
			"#EXTINF:%d tvg-id=\"%s\" tvg-name=\"%s\" group-title=\"%s\",%d %s\n",
			ch.Number, ch.TvgID, ch.Name, ch.Group, ch.Number, ch.Name,
		)
		fmt.Fprintf(&b, "rtsp://%s:%d/?%s\n", publicHost, publicRTSPPort, ch.TuningQuery())
	}
	return b.String()
}

func BuildDeviceDescriptionXML(locationURL string, tunerCount int, profile vendorprofile.Profile) string {
	if tunerCount <= 0 {
		tunerCount = 1
	}
	device := profile.Device
	if device.FriendlyName == "" {
		device = vendorprofile.ForName(vendorprofile.NameGeneric).Device
	}
	modelNumber := ""
	if strings.TrimSpace(device.ModelNumber) != "" {
		modelNumber = fmt.Sprintf("    <modelNumber>%s</modelNumber>\n", xmlEscape(device.ModelNumber))
	}
	return fmt.Sprintf(`<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0" xmlns:satip="urn:ses-com:satip" configId="1">
  <specVersion>
    <major>1</major>
    <minor>0</minor>
  </specVersion>
  <device>
    <deviceType>urn:ses-com:device:SatIPServer:1</deviceType>
    <friendlyName>%s</friendlyName>
    <manufacturer>%s</manufacturer>
    <modelName>%s</modelName>
%s    <UDN>%s</UDN>
    <presentationURL>%s</presentationURL>
    <satip:X_SATIPCAP>DVBS2-%d</satip:X_SATIPCAP>
    <satip:X_SATIPM3U>%s</satip:X_SATIPM3U>
  </device>
</root>
`, xmlEscape(device.FriendlyName), xmlEscape(device.Manufacturer), xmlEscape(device.ModelName), modelNumber, xmlEscape(device.UDN), xmlEscape(locationURL), tunerCount, xmlEscape(device.XSatipM3U))
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}
