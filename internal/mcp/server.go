package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	baseURL string
	client  *http.Client
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func NewServer(baseURL string) *Server {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8875"
	}
	return &Server{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *Server) Handle(req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "satip-lab-mcp",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			resp.Error = &Error{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &Error{Code: -32601, Message: "method not found"}
	}
	return resp
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = encoder.Encode(Response{JSONRPC: "2.0", Error: &Error{Code: -32700, Message: "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		if err := encoder.Encode(s.Handle(req)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) callTool(raw json.RawMessage) (toolCallResult, error) {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return toolCallResult{}, fmt.Errorf("invalid tools/call params: %w", err)
	}
	switch params.Name {
	case "satip_agent_context":
		return s.get("/api/agent/context")
	case "satip_status":
		return s.get("/api/status")
	case "satip_services":
		return s.get("/api/services")
	case "satip_scenario":
		return s.get("/api/scenario")
	case "satip_reset":
		return s.post("/api/reset", nil)
	case "satip_set_scenario":
		return s.post("/api/scenario", params.Arguments)
	case "satip_wait_ready":
		return s.waitReady()
	default:
		return toolCallResult{}, fmt.Errorf("unknown tool %q", params.Name)
	}
}

func (s *Server) get(path string) (toolCallResult, error) {
	resp, err := s.client.Get(s.baseURL + path)
	if err != nil {
		return toolCallResult{}, err
	}
	return readToolResponse(resp)
}

func (s *Server) post(path string, payload any) (toolCallResult, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return toolCallResult{}, err
		}
		body = bytes.NewReader(data)
	}
	resp, err := s.client.Post(s.baseURL+path, "application/json", body)
	if err != nil {
		return toolCallResult{}, err
	}
	return readToolResponse(resp)
}

func (s *Server) waitReady() (toolCallResult, error) {
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		result, err := s.get("/api/agent/context")
		if err == nil {
			return result, nil
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return toolCallResult{}, lastErr
	}
	return toolCallResult{}, fmt.Errorf("satip-lab did not become ready")
}

func readToolResponse(resp *http.Response) (toolCallResult, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return toolCallResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return toolCallResult{}, fmt.Errorf("satip-lab returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return textResult(string(body)), nil
}

func textResult(text string) toolCallResult {
	return toolCallResult{Content: []toolContent{{Type: "text", Text: text}}}
}

func tools() []toolDefinition {
	objectSchema := map[string]any{"type": "object", "properties": map[string]any{}}
	return []toolDefinition{
		{Name: "satip_agent_context", Description: "Return coding-agent bootstrap context from /api/agent/context.", InputSchema: objectSchema},
		{Name: "satip_status", Description: "Return simulator status, including tuners, sessions, and recent events.", InputSchema: objectSchema},
		{Name: "satip_services", Description: "Return the active SAT>IP service catalog.", InputSchema: objectSchema},
		{Name: "satip_scenario", Description: "Return the active runtime scenario.", InputSchema: objectSchema},
		{Name: "satip_reset", Description: "Reset sessions, tuners, RTP senders, and lab events.", InputSchema: objectSchema},
		{Name: "satip_set_scenario", Description: "Set a runtime scenario; accepts name plus optional service_id, mux_id, duration_min, or a timeline array.", InputSchema: scenarioInputSchema()},
		{Name: "satip_wait_ready", Description: "Poll /api/agent/context until the simulator is ready.", InputSchema: objectSchema},
	}
}

func scenarioInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":         map[string]any{"type": "string"},
			"service_id":   map[string]any{"type": "string"},
			"mux_id":       map[string]any{"type": "string"},
			"duration_min": map[string]any{"type": "integer"},
			"timeline": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"at_ms", "name"},
					"properties": map[string]any{
						"at_ms":        map[string]any{"type": "integer"},
						"name":         map[string]any{"type": "string"},
						"service_id":   map[string]any{"type": "string"},
						"mux_id":       map[string]any{"type": "string"},
						"duration_min": map[string]any{"type": "integer"},
					},
				},
			},
		},
	}
}
