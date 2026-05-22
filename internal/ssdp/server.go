package ssdp

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/e12media/satip-lab/internal/config"
	"golang.org/x/net/ipv4"
)

type Server struct {
	cfg       config.Config
	conn      *net.UDPConn
	stopCh    chan struct{}
	multicast *net.UDPAddr
}

func New(cfg config.Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", s.cfg.SSDPort))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	s.conn = conn

	mcast, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return err
	}
	s.multicast = mcast
	if err := ipv4.NewPacketConn(conn).JoinGroup(nil, mcast); err != nil {
		return err
	}

	s.stopCh = make(chan struct{})
	go s.readLoop()
	go s.notifyLoop()
	return nil
}

func (s *Server) Stop() {
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

func (s *Server) readLoop() {
	buf := make([]byte, 4096)
	for {
		if s.stopCh == nil {
			return
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, remote, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				return
			}
		}
		if isSatIPSearch(string(buf[:n])) {
			_, _ = s.conn.WriteToUDP([]byte(s.searchResponse()), remote)
		}
	}
}

func (s *Server) notifyLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			msg := []byte(s.notifyMessage())
			_, _ = s.conn.WriteToUDP(msg, s.multicast)
		}
	}
}

func isSatIPSearch(payload string) bool {
	upper := strings.ToUpper(payload)
	if !strings.Contains(upper, "M-SEARCH") {
		return false
	}
	return strings.Contains(upper, strings.ToUpper(config.SatIPSearchTarget)) ||
		strings.Contains(upper, "SES-COM:DEVICE:SATIPSERVER")
}

func (s *Server) searchResponse() string {
	profile := s.cfg.CompatibilityProfile()
	return strings.Join([]string{
		"HTTP/1.1 200 OK",
		"CACHE-CONTROL: max-age=1800",
		"EXT:",
		"LOCATION: " + s.cfg.DeviceDescriptionURL(),
		"CONFIGID.UPNP.ORG: 1",
		"SERVER: " + profile.SSDP.Server,
		"ST: " + profile.SSDP.ST,
		"USN: " + profile.SSDP.USN,
		"", "",
	}, "\r\n")
}

func (s *Server) notifyMessage() string {
	profile := s.cfg.CompatibilityProfile()
	return strings.Join([]string{
		"NOTIFY * HTTP/1.1",
		"HOST: 239.255.255.250:1900",
		"CACHE-CONTROL: max-age=1800",
		"LOCATION: " + s.cfg.DeviceDescriptionURL(),
		"CONFIGID.UPNP.ORG: 1",
		"NT: " + profile.SSDP.ST,
		"NTS: ssdp:alive",
		"SERVER: " + profile.SSDP.Server,
		"USN: " + profile.SSDP.USN,
		"", "",
	}, "\r\n")
}
