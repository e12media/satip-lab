package smoke

import (
	"bufio"
	"context"
	"encoding/json"
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
	Profile        string
	Timeout        time.Duration
}

type RTSPProbeResult struct {
	SessionID       string
	SessionIDFormat string
	Profile         string
	RTPBytes        int
	PayloadType     byte
	MPEGTSSyncByte  byte
	RTSPExchanges   []RTSPExchange
}

type RTSPExchange struct {
	Method     string
	StatusLine string
	Headers    map[string]string
	Duration   time.Duration
}

type probeEvidence struct {
	Profile         string             `json:"profile,omitempty"`
	SessionID       string             `json:"session_id"`
	SessionIDFormat string             `json:"session_id_format"`
	RTP             rtpEvidence        `json:"rtp"`
	RTSP            []jsonRTSPExchange `json:"rtsp"`
}

type rtpEvidence struct {
	Bytes          int    `json:"bytes"`
	PayloadType    byte   `json:"payload_type"`
	MPEGTSSyncByte string `json:"mpeg_ts_sync_byte"`
}

type jsonRTSPExchange struct {
	Method     string            `json:"method"`
	StatusLine string            `json:"status_line"`
	Headers    map[string]string `json:"headers"`
	DurationMS int64             `json:"duration_ms"`
}

func (r RTSPProbeResult) JSONEvidence() ([]byte, error) {
	exchanges := make([]jsonRTSPExchange, 0, len(r.RTSPExchanges))
	for _, exchange := range r.RTSPExchanges {
		exchanges = append(exchanges, jsonRTSPExchange{
			Method:     exchange.Method,
			StatusLine: exchange.StatusLine,
			Headers:    exchange.Headers,
			DurationMS: exchange.Duration.Milliseconds(),
		})
	}
	doc := probeEvidence{
		Profile:         r.Profile,
		SessionID:       r.SessionID,
		SessionIDFormat: r.SessionIDFormat,
		RTP: rtpEvidence{
			Bytes:          r.RTPBytes,
			PayloadType:    r.PayloadType,
			MPEGTSSyncByte: fmt.Sprintf("0x%02x", r.MPEGTSSyncByte),
		},
		RTSP: exchanges,
	}
	return json.MarshalIndent(doc, "", "  ")
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

	setupExchange, err := rtspExchangeEvidence(conn, "SETUP", fmt.Sprintf(
		"SETUP rtsp://%s:%d/?src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102 RTSP/1.0\r\n"+
			"CSeq: 1\r\nTransport: %s\r\n\r\n",
		opts.Host, opts.Port, strings.Join(transportParts, ";"),
	))
	if err != nil {
		return RTSPProbeResult{}, err
	}
	setup := setupExchange.StatusLine + "\r\n" + headersForError(setupExchange.Headers)
	if !strings.Contains(setup, "200 OK") {
		return RTSPProbeResult{}, fmt.Errorf("SETUP failed: %s", strings.TrimSpace(setup))
	}
	sessionID := setupExchange.Headers["Session"]
	if sessionID == "" {
		sessionID = sessionIDFromResponse(setup)
	}
	if sessionID == "" {
		return RTSPProbeResult{}, fmt.Errorf("SETUP response missing Session header")
	}

	playExchange, err := rtspExchangeEvidence(conn, "PLAY", fmt.Sprintf(
		"PLAY rtsp://%s:%d/ RTSP/1.0\r\nCSeq: 2\r\nSession: %s\r\n\r\n",
		opts.Host, opts.Port, sessionID,
	))
	if err != nil {
		return RTSPProbeResult{}, err
	}
	play := playExchange.StatusLine
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

	teardownExchange, _ := rtspExchangeEvidence(conn, "TEARDOWN", fmt.Sprintf(
		"TEARDOWN rtsp://%s:%d/ RTSP/1.0\r\nCSeq: 3\r\nSession: %s\r\n\r\n",
		opts.Host, opts.Port, sessionID,
	))

	return RTSPProbeResult{
		SessionID:       sessionID,
		SessionIDFormat: sessionIDFormat(sessionID),
		Profile:         opts.Profile,
		RTPBytes:        n,
		PayloadType:     payloadType,
		MPEGTSSyncByte:  buf[12],
		RTSPExchanges:   []RTSPExchange{setupExchange, playExchange, teardownExchange},
	}, nil
}

func rtspExchangeEvidence(conn net.Conn, method, request string) (RTSPExchange, error) {
	start := time.Now()
	response, err := rtspExchange(conn, request)
	if err != nil {
		return RTSPExchange{}, err
	}
	exchange := parseRTSPExchange(method, response)
	exchange.Duration = time.Since(start)
	return exchange, nil
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

func parseRTSPExchange(method, response string) RTSPExchange {
	lines := strings.Split(response, "\r\n")
	exchange := RTSPExchange{
		Method:  method,
		Headers: make(map[string]string),
	}
	if len(lines) > 0 {
		exchange.StatusLine = lines[0]
	}
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if strings.EqualFold(key, "Session") {
			value = sessionIDFromResponse(key + ": " + value)
		}
		exchange.Headers[key] = value
	}
	return exchange
}

func sessionIDFormat(sessionID string) string {
	if sessionID == "" {
		return "empty"
	}
	for _, r := range sessionID {
		if r < '0' || r > '9' {
			return "opaque"
		}
	}
	return "numeric"
}

func headersForError(headers map[string]string) string {
	var b strings.Builder
	for key, value := range headers {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\r\n")
	}
	return b.String()
}
