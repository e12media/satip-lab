package simulator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/e12media/satip-lab/internal/channels"
	"github.com/e12media/satip-lab/internal/config"
	"github.com/e12media/satip-lab/internal/httpserver"
	"github.com/e12media/satip-lab/internal/lab"
	"github.com/e12media/satip-lab/internal/rtsp"
	"github.com/e12media/satip-lab/internal/ssdp"
	"github.com/e12media/satip-lab/internal/topology"
	"github.com/e12media/satip-lab/internal/ts"
)

type Simulator struct {
	cfg  config.Config
	http *httpserver.Server
	rtsp *rtsp.Server
	ssdp *ssdp.Server
	lab  *lab.Manager
}

func New(cfg config.Config) (*Simulator, error) {
	source := &ts.Source{Path: cfg.TransportStreamPath, SampleProfile: cfg.SampleProfile}
	catalog := lab.DefaultCatalog()
	if strings.TrimSpace(cfg.CatalogPath) != "" {
		list, err := channels.LoadCatalogFile(cfg.CatalogPath)
		if err != nil {
			return nil, err
		}
		catalog = lab.CatalogFromChannels(list)
	}
	topologyDoc := httpserver.DefaultTopology(cfg)
	if strings.TrimSpace(cfg.TopologyPath) != "" {
		doc, err := topology.LoadFile(cfg.TopologyPath)
		if err != nil {
			return nil, err
		}
		topologyDoc = doc
	}
	labManager := lab.NewManager(catalog, cfg.TunerCount)
	rtspServer := rtsp.NewServer(cfg, source, labManager)
	return &Simulator{
		cfg:  cfg,
		http: httpserver.NewWithTopology(cfg, labManager, topologyDoc, rtspServer.Reset),
		rtsp: rtspServer,
		ssdp: ssdp.New(cfg),
		lab:  labManager,
	}, nil
}

func (s *Simulator) Start() error {
	if err := s.http.Start(); err != nil {
		return err
	}
	if err := s.rtsp.Start(); err != nil {
		return err
	}
	if s.cfg.SSDPort > 0 {
		if err := s.ssdp.Start(); err != nil {
			fmt.Fprintf(os.Stderr,
				"Warning: SSDP on port %d unavailable (%v). Use manual server IP %s instead.\n",
				s.cfg.SSDPort, err, s.cfg.HTTPBaseURL(),
			)
		}
	}
	return nil
}

func (s *Simulator) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.ssdp.Stop()
	_ = s.rtsp.Stop()
	return s.http.Stop(ctx)
}
