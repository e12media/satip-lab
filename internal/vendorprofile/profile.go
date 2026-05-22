package vendorprofile

import "strings"

const (
	NameGeneric      = "generic-satip-1.2"
	NameSpec         = "spec"
	SessionIDNumeric = "numeric"
)

type DeviceProfile struct {
	FriendlyName    string
	Manufacturer    string
	ModelName       string
	ModelNumber     string
	UDN             string
	DescriptionPath string
	XSatipM3U       string
}

type SSDPProfile struct {
	Server string
	ST     string
	USN    string
}

type Evidence struct {
	Confidence string
	Sources    []string
	Notes      []string
}

type Profile struct {
	Name                       string
	Device                     DeviceProfile
	SSDP                       SSDPProfile
	Evidence                   Evidence
	SessionHeader              string
	TransportHeader            string
	SessionIDFormat            string
	IncludeSetupTimeout        bool
	RequireDescribeBeforeSetup bool
	TunerBusyStatus            string
}

var genericProfile = Profile{
	Name: NameGeneric,
	Device: DeviceProfile{
		FriendlyName:    "satip-lab",
		Manufacturer:    "e12media",
		ModelName:       "SAT>IP Lab Server",
		UDN:             "uuid:satip-lab-dev",
		DescriptionPath: "/desc.xml",
		XSatipM3U:       "/channels.m3u",
	},
	SSDP: SSDPProfile{
		Server: "satip-lab/0.2 UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:satip-lab-dev::urn:ses-com:device:SatIPServer:1",
	},
	Evidence: Evidence{
		Confidence: "spec",
		Sources:    []string{"SAT>IP protocol baseline", "satip-lab supported-profile.md"},
		Notes:      []string{"Default simulator profile. Non-spec behavior remains disabled."},
	},
	SessionHeader:       "Session",
	TransportHeader:     "Transport",
	SessionIDFormat:     SessionIDNumeric,
	IncludeSetupTimeout: true,
	TunerBusyStatus:     "503 Service Unavailable",
}

var specProfile = withName(genericProfile, NameSpec)

var profiles = []Profile{
	genericProfile,
	specProfile,
	withDevice("minisatip", DeviceProfile{
		FriendlyName:    "minisatip",
		Manufacturer:    "minisatip",
		ModelName:       "minisatip SAT>IP",
		UDN:             "uuid:minisatip-lab",
		DescriptionPath: "/desc.xml",
		XSatipM3U:       "/channels.m3u",
	}, SSDPProfile{
		Server: "minisatip/1.0 UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:minisatip-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"minisatip public source/docs"}),
	withDevice("tvheadend", DeviceProfile{
		FriendlyName:    "TVHeadend SAT>IP",
		Manufacturer:    "TVHeadend",
		ModelName:       "TVHeadend SAT>IP",
		UDN:             "uuid:tvheadend-satip-lab",
		DescriptionPath: "/satip_server/desc.xml",
		XSatipM3U:       "/channellist.m3u",
	}, SSDPProfile{
		Server: "TVHeadend UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:tvheadend-satip-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"Tvheadend SAT>IP docs and public issue examples"}),
	withDevice("triax-tss400", DeviceProfile{
		FriendlyName:    "TRIAX TSS 400",
		Manufacturer:    "TRIAX",
		ModelName:       "TSS 400",
		UDN:             "uuid:triax-tss400-lab",
		DescriptionPath: "/desc.xml",
		XSatipM3U:       "/channels.m3u",
	}, SSDPProfile{
		Server: "TRIAX SATIPServer UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:triax-tss400-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"Vendor product/manual metadata; protocol quirks require captured traces"}),
	withDevice("telestar-digibit-r1", DeviceProfile{
		FriendlyName:    "TELestar DIGIBIT R1",
		Manufacturer:    "TELestar",
		ModelName:       "DIGIBIT R1",
		UDN:             "uuid:telestar-digibit-r1-lab",
		DescriptionPath: "/desc.xml",
		XSatipM3U:       "/channels.m3u",
	}, SSDPProfile{
		Server: "TELestar SATIPServer UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:telestar-digibit-r1-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"Vendor product/manual metadata; satip-axe community references need trace promotion"}),
	withDevice("kathrein-exip", DeviceProfile{
		FriendlyName:    "Kathrein EXIP",
		Manufacturer:    "Kathrein",
		ModelName:       "EXIP",
		UDN:             "uuid:kathrein-exip-lab",
		DescriptionPath: "/desc.xml",
		XSatipM3U:       "/channels.m3u",
	}, SSDPProfile{
		Server: "Kathrein SATIPServer UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:kathrein-exip-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"Vendor product/manual metadata; protocol quirks require captured traces"}),
	withDevice("digital-devices-octopus-net", DeviceProfile{
		FriendlyName:    "Digital Devices Octopus NET",
		Manufacturer:    "Digital Devices",
		ModelName:       "Octopus NET",
		UDN:             "uuid:digital-devices-octopus-net-lab",
		DescriptionPath: "/octoserve/octonet.xml",
		XSatipM3U:       "/channels.m3u",
	}, SSDPProfile{
		Server: "Digital Devices SATIPServer UPnP/1.0",
		ST:     "urn:ses-com:device:SatIPServer:1",
		USN:    "uuid:digital-devices-octopus-net-lab::urn:ses-com:device:SatIPServer:1",
	}, "public-doc", []string{"Vendor support/product metadata; protocol quirks require captured traces"}),
}

func ForName(name string) Profile {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return genericProfile
	}
	for _, profile := range profiles {
		if profile.Name == normalized {
			return profile
		}
	}
	return specProfile
}

func Names() []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.Name)
	}
	return names
}

func withName(profile Profile, name string) Profile {
	profile.Name = name
	return profile
}

func withDevice(name string, device DeviceProfile, ssdp SSDPProfile, confidence string, sources []string) Profile {
	profile := genericProfile
	profile.Name = name
	profile.Device = device
	profile.SSDP = ssdp
	profile.Evidence = Evidence{
		Confidence: confidence,
		Sources:    sources,
		Notes:      []string{"Only advertised metadata and documented generic SAT>IP behavior are simulated until trace-backed quirks are added."},
	}
	return profile
}
