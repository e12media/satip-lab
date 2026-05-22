package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/simulator"
)

func main() {
	defaults := config.FromEnvironment()

	bind := flag.String("bind", defaults.BindAddress, "bind address")
	publicHost := flag.String("public-host", defaults.PublicHost, "host advertised in SSDP LOCATION and M3U")
	httpPort := flag.Int("http-port", defaults.HTTPPort, "HTTP port")
	rtspPort := flag.Int("rtsp-port", defaults.RTSPPort, "RTSP port")
	publicHTTPPort := flag.Int("public-http-port", defaults.PublicHTTPPort, "HTTP port advertised in SSDP LOCATION (0 uses --http-port)")
	publicRTSPPort := flag.Int("public-rtsp-port", defaults.PublicRTSPPort, "RTSP port advertised in M3U URLs (0 uses --rtsp-port)")
	tunerCount := flag.Int("tuners", defaults.TunerCount, "number of simulated SAT>IP tuners")
	ssdpPort := flag.Int("ssdp-port", defaults.SSDPort, "SSDP UDP port (0 disables)")
	catalogPath := flag.String("catalog", defaults.CatalogPath, "YAML channel catalog path")
	tsPath := flag.String("ts-path", defaults.TransportStreamPath, "MPEG-TS file looped for RTP")
	sampleProfile := flag.String("sample-profile", defaults.SampleProfile, "sample profile: synthetic, h264_aac_short, or h264_silent")
	profile := flag.String("profile", defaults.Profile, "compatibility profile for SSDP, device XML, M3U, and RTSP")
	vendorProfile := flag.String("vendor-profile", defaults.VendorProfile, "RTSP behavior profile selector alias")
	epgClock := flag.String("epg-clock", defaults.EPGClock, "EPG clock: fixed:<rfc3339> or real")
	scenario := flag.String("scenario", defaults.ScenarioName(), "normal or tuner_busy")
	flag.Parse()
	activeProfile := *profile
	profileFlagSet := false
	vendorProfileFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "profile" {
			profileFlagSet = true
		}
		if f.Name == "vendor-profile" {
			vendorProfileFlagSet = true
		}
	})
	if vendorProfileFlagSet && !profileFlagSet {
		activeProfile = *vendorProfile
	}

	cfg := config.Config{
		BindAddress:         *bind,
		PublicHost:          *publicHost,
		HTTPPort:            *httpPort,
		RTSPPort:            *rtspPort,
		PublicHTTPPort:      *publicHTTPPort,
		PublicRTSPPort:      *publicRTSPPort,
		TunerCount:          *tunerCount,
		SSDPort:             *ssdpPort,
		CatalogPath:         *catalogPath,
		TransportStreamPath: *tsPath,
		SampleProfile:       *sampleProfile,
		Profile:             activeProfile,
		VendorProfile:       *vendorProfile,
		EPGClock:            *epgClock,
		Scenario:            config.ScenarioNormal,
	}
	if *scenario == "tuner_busy" {
		cfg.Scenario = config.ScenarioTunerBusy
	}

	sim, err := simulator.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "satip-lab failed to configure: %v\n", err)
		os.Exit(1)
	}
	if err := sim.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "satip-lab failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("satip-lab running")
	fmt.Println("  SSDP target :", config.SatIPSearchTarget)
	fmt.Println("  Device XML  :", cfg.DeviceDescriptionURL())
	fmt.Println("  Channel M3U :", cfg.M3UURL())
	fmt.Printf("  RTSP        : rtsp://%s:%d/\n", cfg.PublicHost, cfg.EffectivePublicRTSPPort())
	if cfg.EffectivePublicHTTPPort() != cfg.HTTPPort || cfg.EffectivePublicRTSPPort() != cfg.RTSPPort {
		fmt.Printf("  Listen ports: HTTP %d, RTSP %d\n", cfg.HTTPPort, cfg.RTSPPort)
	}
	fmt.Println("  Scenario    :", cfg.ScenarioName())
	fmt.Println("  Tuners      :", cfg.TunerCount)
	fmt.Println("  Catalog     :", cfg.CatalogPath)
	fmt.Println("  TS file     :", cfg.TransportStreamPath)
	fmt.Println("  Sample prof.:", cfg.SampleProfile)
	fmt.Println("  Profile     :", cfg.CompatibilityProfile().Name)
	fmt.Println("  Vendor prof.:", cfg.VendorProfile)
	fmt.Println("  EPG clock   :", cfg.EPGClock)
	if cfg.SSDPort <= 0 {
		fmt.Println("  SSDP        : disabled")
	}
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	_ = sim.Stop()
}
