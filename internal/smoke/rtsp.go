package smoke

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

var sessionPattern = regexp.MustCompile(`(?i)Session:\s*([^;\r\n]+)`)

type RTSPProbeOptions struct {
	Host           string
	Port           int
	RTPBindAddress string
	Destination    string
	Timeout        time.Duration
}

type RTSPProbeResult struct {
	SessionID      string
	RTPBytes       int
	PayloadType    byte
	MPEGTSSyncByte byte
}

func ProbeRTSPRTP(ctx context.Context, opts RTSPProbeOptions) (RTSPProbeResult, error) {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.Port == 0 {
		opts.Port = 554
	}
	if opts.RTPBindAddress == "" {
		opts.RTPBindAddress = "0.0.0.0"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	rtpConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(opts.RTPBindAddress), Port: 0})
	if err != nil {
		return RTSPProbeResult{}, err
	}
	defer rtpConn.Close()
	rtpPort := rtpConn.LocalAddr().(*net.UDPAddr).Port

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
	if err != nil {
		return RTSPProbeResult{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
		_ = rtpConn.SetReadDeadline(deadline)
	}

	transportParts := []string{
		"RTP/AVP",
		"unicast",
		fmt.Sprintf("client_port=%d-%d", rtpPort, rtpPort+1),
	}
	if opts.Destination != "" {
		transportParts = append(transportParts, "destination="+opts.Destination)
	}

	setup, err := rtspExchange(conn, fmt.Sprintf(
		"SETUP rtsp://%s:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: %s\r\n\r\n",
		opts.Host, opts.Port, strings.Join(transportParts, ";"),
	))
	if err != nil {
		return RTSPProbeResult{}, err
	}
	if !strings.Contains(setup, "200 OK") {
		return RTSPProbeResult{}, fmt.Errorf("SETUP failed: %s", strings.TrimSpace(setup))
	}
	sessionID := sessionIDFromResponse(setup)
	if sessionID == "" {
		return RTSPProbeResult{}, fmt.Errorf("SETUP response missing Session header")
	}

	play, err := rtspExchange(conn, fmt.Sprintf(
		"PLAY rtsp://%s:%d/ RTSP/1.0\r\nCSeq: 2\r\nSession: %s\r\n\r\n",
		opts.Host, opts.Port, sessionID,
	))
	if err != nil {
		return RTSPProbeResult{}, err
	}
	if !strings.Contains(play, "200 OK") {
		return RTSPProbeResult{}, fmt.Errorf("PLAY failed: %s", strings.TrimSpace(play))
	}

	buf := make([]byte, 2048)
	n, _, err := rtpConn.ReadFromUDP(buf)
	if err != nil {
		return RTSPProbeResult{}, err
	}
	if n < 13 {
		return RTSPProbeResult{}, fmt.Errorf("RTP packet too small: %d bytes", n)
	}
	if got := buf[0] >> 6; got != 2 {
		return RTSPProbeResult{}, fmt.Errorf("unexpected RTP version %d", got)
	}
	payloadType := buf[1] & 0x7F
	if payloadType != 33 {
		return RTSPProbeResult{}, fmt.Errorf("unexpected RTP payload type %d", payloadType)
	}
	if buf[12] != 0x47 {
		return RTSPProbeResult{}, fmt.Errorf("unexpected MPEG-TS sync byte 0x%x", buf[12])
	}

	_, _ = rtspExchange(conn, fmt.Sprintf(
		"TEARDOWN rtsp://%s:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		opts.Host, opts.Port, sessionID,
	))

	return RTSPProbeResult{
		SessionID:      sessionID,
		RTPBytes:       n,
		PayloadType:    payloadType,
		MPEGTSSyncByte: buf[12],
	}, nil
}

func rtspExchange(conn net.Conn, request string) (string, error) {
	if _, err := conn.Write([]byte(request)); err != nil {
		return "", err
	}
	reader := bufio.NewReader(conn)
	var buf strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		buf.WriteString(line)
		if strings.HasSuffix(buf.String(), "\r\n\r\n") {
			return buf.String(), nil
		}
	}
}

func sessionIDFromResponse(response string) string {
	match := sessionPattern.FindStringSubmatch(response)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
