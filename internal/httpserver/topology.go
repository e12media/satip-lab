package httpserver

import (
	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/topology"
)

func DefaultTopology(cfg config.Config) topology.Document {
	return topologyDocument(cfg)
}

func topologyDocument(cfg config.Config) topology.Document {
	profile := cfg.CompatibilityProfile()
	publicHost := cfg.PublicHost
	if publicHost == "" {
		publicHost = "127.0.0.1"
	}
	httpPort := cfg.EffectivePublicHTTPPort()
	if httpPort == 0 {
		httpPort = 8875
	}
	rtspPort := cfg.EffectivePublicRTSPPort()
	if rtspPort == 0 {
		rtspPort = 554
	}
	tuners := cfg.TunerCount
	if tuners == 0 {
		tuners = 2
	}
	doc := topology.Document{
		Devices: []topology.Device{
			{
				ID:           "default",
				FriendlyName: profile.Device.FriendlyName,
				Profile:      profile.Name,
				PublicHost:   publicHost,
				HTTPPort:     httpPort,
				RTSPPort:     rtspPort,
				Tuners:       tuners,
			},
		},
	}
	_ = doc.NormalizeAndValidate()
	return doc
}
