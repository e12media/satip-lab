package simctl

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/e12media/satip-lab/internal/smoke"
)

type command struct {
	httpURL string
	client  *http.Client
	stdout  io.Writer
	stderr  io.Writer
}

func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("satip-labctl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	httpURL := fs.String("http-url", "http://127.0.0.1:8875", "satip-lab HTTP base URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		usage(stderr)
		return 2
	}

	cmd := command{
		httpURL: strings.TrimRight(*httpURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
		stdout:  stdout,
		stderr:  stderr,
	}
	if err := cmd.run(rest); err != nil {
		fmt.Fprintf(stderr, "satip-labctl: %v\n", err)
		return 1
	}
	return 0
}

func (c command) run(args []string) error {
	switch args[0] {
	case "context":
		return c.get("/api/agent/context")
	case "status":
		return c.get("/api/status")
	case "services":
		return c.get("/api/services")
	case "scenario":
		return c.scenario(args[1:])
	case "reset":
		return c.post("/api/reset", nil)
	case "wait":
		return c.wait(args[1:])
	case "smoke":
		return c.smoke(args[1:])
	case "help", "-h", "--help":
		usage(c.stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (c command) get(path string) error {
	resp, err := c.client.Get(c.httpURL + path)
	if err != nil {
		return err
	}
	return writeResponse(c.stdout, resp)
}

func (c command) post(path string, payload any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	resp, err := c.client.Post(c.httpURL+path, "application/json", body)
	if err != nil {
		return err
	}
	return writeResponse(c.stdout, resp)
}

func (c command) scenario(args []string) error {
	if len(args) == 0 {
		return c.get("/api/scenario")
	}
	if isHelpArg(args[0]) {
		scenarioUsage(c.stdout)
		return nil
	}
	fs := flag.NewFlagSet("scenario", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	serviceID := fs.String("service", "", "service id target")
	muxID := fs.String("mux", "", "mux id target")
	durationMin := fs.Int("duration-min", 0, "duration in minutes for epg_gap")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	payload := map[string]any{"name": args[0]}
	if *serviceID != "" {
		payload["service_id"] = *serviceID
	}
	if *muxID != "" {
		payload["mux_id"] = *muxID
	}
	if *durationMin > 0 {
		payload["duration_min"] = *durationMin
	}
	return c.post("/api/scenario", payload)
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func (c command) wait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	timeout := fs.Duration("timeout", 30*time.Second, "maximum time to wait")
	interval := fs.Duration("interval", 250*time.Millisecond, "poll interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	deadline := time.Now().Add(*timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := c.client.Get(c.httpURL + "/api/agent/context")
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			_ = resp.Body.Close()
			fmt.Fprintln(c.stdout, "satip-lab ready")
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("status %s", resp.Status)
		} else {
			lastErr = err
		}
		time.Sleep(*interval)
	}
	if lastErr != nil {
		return fmt.Errorf("wait timed out: %w", lastErr)
	}
	return fmt.Errorf("wait timed out")
}

func (c command) smoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	host := fs.String("rtsp-host", "127.0.0.1", "RTSP host")
	port := fs.Int("rtsp-port", 554, "RTSP port")
	rtpBind := fs.String("rtp-bind", "0.0.0.0", "local address for RTP UDP listener")
	destination := fs.String("rtp-destination", "", "destination IP to include in RTSP Transport header")
	timeout := fs.Duration("timeout", 5*time.Second, "probe timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := smoke.ProbeRTSPRTP(context.Background(), smoke.RTSPProbeOptions{
		Host:           *host,
		Port:           *port,
		RTPBindAddress: *rtpBind,
		Destination:    *destination,
		Timeout:        *timeout,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "RTSP/RTP smoke OK: session=%s rtp_bytes=%d payload_type=%d sync=0x%02x\n",
		result.SessionID, result.RTPBytes, result.PayloadType, result.MPEGTSSyncByte)
	return nil
}

func writeResponse(out io.Writer, resp *http.Response) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("satip-lab returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	_, err = out.Write(body)
	if err == nil && len(body) > 0 && body[len(body)-1] != '\n' {
		_, err = fmt.Fprintln(out)
	}
	return err
}

func usage(out io.Writer) {
	fmt.Fprintln(out, `Usage: satip-labctl [--http-url URL] <command> [options]

Commands:
  wait       Poll /api/agent/context until satip-lab is ready
  context    Print /api/agent/context
  status     Print /api/status
  services   Print /api/services
  scenario   Get or set runtime scenario
  reset      POST /api/reset
  smoke      Run RTSP/RTP smoke probe`)
}

func scenarioUsage(out io.Writer) {
	fmt.Fprintln(out, `Usage: satip-labctl [--http-url URL] scenario [name] [options]

Without a name, prints the active runtime scenario.

Options:
  --service string       service id target
  --mux string           mux id target
  --duration-min int     duration in minutes for epg_gap`)
}
