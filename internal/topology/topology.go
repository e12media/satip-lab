package topology

import (
	"fmt"
	"os"
	"strings"

	"github.com/e12media/satip-lab/internal/vendorprofile"
	"gopkg.in/yaml.v3"
)

type Document struct {
	Devices []Device `json:"devices" yaml:"devices"`
}

type Device struct {
	ID              string `json:"id" yaml:"id"`
	FriendlyName    string `json:"friendly_name" yaml:"friendly_name"`
	Profile         string `json:"profile" yaml:"profile"`
	PublicHost      string `json:"public_host" yaml:"public_host"`
	HTTPPort        int    `json:"http_port" yaml:"http_port"`
	RTSPPort        int    `json:"rtsp_port" yaml:"rtsp_port"`
	Tuners          int    `json:"tuners" yaml:"tuners"`
	Location        string `json:"location" yaml:"location"`
	StaleLocation   bool   `json:"stale_location" yaml:"stale_location"`
	DescriptionPath string `json:"description_path" yaml:"description_path"`
}

func LoadFile(path string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Document{}, err
	}
	if err := doc.NormalizeAndValidate(); err != nil {
		return Document{}, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

func (d *Document) NormalizeAndValidate() error {
	if len(d.Devices) == 0 {
		return fmt.Errorf("devices: must contain at least one device")
	}
	seen := make(map[string]struct{}, len(d.Devices))
	for i := range d.Devices {
		device := &d.Devices[i]
		device.ID = strings.TrimSpace(device.ID)
		if device.ID == "" {
			return fmt.Errorf("devices[%d].id: required", i)
		}
		if _, ok := seen[device.ID]; ok {
			return fmt.Errorf("duplicate device id %q", device.ID)
		}
		seen[device.ID] = struct{}{}
		device.Profile = strings.TrimSpace(device.Profile)
		if device.Profile == "" {
			device.Profile = vendorprofile.NameGeneric
		}
		profile, ok := profileByName(device.Profile)
		if !ok {
			return fmt.Errorf("devices[%d].profile: unknown profile %q", i, device.Profile)
		}
		device.Profile = profile.Name
		device.FriendlyName = strings.TrimSpace(device.FriendlyName)
		if device.FriendlyName == "" {
			device.FriendlyName = profile.Device.FriendlyName
		}
		device.PublicHost = strings.TrimSpace(device.PublicHost)
		if device.PublicHost == "" {
			return fmt.Errorf("devices[%d].public_host: required", i)
		}
		if device.HTTPPort <= 0 {
			return fmt.Errorf("devices[%d].http_port: must be positive", i)
		}
		if device.RTSPPort <= 0 {
			return fmt.Errorf("devices[%d].rtsp_port: must be positive", i)
		}
		if device.Tuners <= 0 {
			return fmt.Errorf("devices[%d].tuners: must be positive", i)
		}
		device.DescriptionPath = profile.Device.DescriptionPath
		if strings.TrimSpace(device.Location) == "" {
			device.Location = fmt.Sprintf("http://%s:%d%s", device.PublicHost, device.HTTPPort, device.DescriptionPath)
		}
	}
	return nil
}

func profileByName(name string) (vendorprofile.Profile, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, candidate := range vendorprofile.Names() {
		if candidate == normalized {
			return vendorprofile.ForName(candidate), true
		}
	}
	return vendorprofile.Profile{}, false
}
