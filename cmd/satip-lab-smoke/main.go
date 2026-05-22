package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/e12media/satip-lab/internal/smoke"
)

func main() {
	host := flag.String("host", "127.0.0.1", "RTSP host")
	port := flag.Int("rtsp-port", 554, "RTSP port")
	rtpBind := flag.String("rtp-bind", "0.0.0.0", "local address for RTP UDP listener")
	destination := flag.String("rtp-destination", "", "destination IP to include in RTSP Transport header")
	timeout := flag.Duration("timeout", 5*time.Second, "probe timeout")
	flag.Parse()

	result, err := smoke.ProbeRTSPRTP(context.Background(), smoke.RTSPProbeOptions{
		Host:           *host,
		Port:           *port,
		RTPBindAddress: *rtpBind,
		Destination:    *destination,
		Timeout:        *timeout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "RTSP/RTP smoke failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("RTSP/RTP smoke OK: session=%s rtp_bytes=%d payload_type=%d sync=0x%02x\n",
		result.SessionID, result.RTPBytes, result.PayloadType, result.MPEGTSSyncByte)
}
