package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerListsSatIPTools(t *testing.T) {
	server := NewServer("http://satip.example")

	response := server.Handle(Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	})

	if response.Error != nil {
		t.Fatalf("tools/list error: %+v", response.Error)
	}
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	mustDecodeResult(t, response, &result)
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{"satip_status", "satip_reset", "satip_set_scenario", "satip_agent_context", "satip_wait_ready"} {
		if !containsString(names, want) {
			t.Fatalf("missing tool %q in %#v", want, names)
		}
	}
}

func TestServerHandlesPing(t *testing.T) {
	server := NewServer("http://satip.example")

	response := server.Handle(Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "ping",
	})

	if response.Error != nil {
		t.Fatalf("ping error: %+v", response.Error)
	}
	result, ok := response.Result.(map[string]any)
	if !ok || len(result) != 0 {
		t.Fatalf("ping result: %#v", response.Result)
	}
}

func TestServerServeSupportsBasicMCPFlow(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tuners":[],"sessions":[],"events":[]}`))
	}))
	defer api.Close()
	server := NewServer(api.URL)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"satip_status","arguments":{}}}`,
		"",
	}, "\n")
	var output bytes.Buffer

	if err := server.Serve(t.Context(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("response lines: got %d\n%s", len(lines), output.String())
	}
	if !strings.Contains(lines[1], `"id":2`) || !strings.Contains(lines[1], `"result":{}`) {
		t.Fatalf("ping response: %s", lines[1])
	}
	if strings.Contains(output.String(), "notifications/initialized") {
		t.Fatalf("notification should not produce a response: %s", output.String())
	}
}

func TestServerCallsStatusTool(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tuners":[{"id":1,"state":"idle"}],"sessions":[],"events":[]}`))
	}))
	defer api.Close()
	server := NewServer(api.URL)

	response := server.Handle(Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`2`),
		Method:  "tools/call",
		Params:  mustRaw(`{"name":"satip_status","arguments":{}}`),
	})

	if response.Error != nil {
		t.Fatalf("satip_status error: %+v", response.Error)
	}
	var result toolCallResult
	mustDecodeResult(t, response, &result)
	if len(result.Content) != 1 || result.Content[0].Type != "text" || !strings.Contains(result.Content[0].Text, `"tuners"`) {
		t.Fatalf("unexpected content: %+v", result.Content)
	}
}

func TestServerCallsSetScenarioTool(t *testing.T) {
	var gotMethod string
	var gotBody bytes.Buffer
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody.ReadFrom(r.Body)
		if r.URL.Path != "/api/scenario" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"rtp_loss"}`))
	}))
	defer api.Close()
	server := NewServer(api.URL)

	response := server.Handle(Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`3`),
		Method:  "tools/call",
		Params:  mustRaw(`{"name":"satip_set_scenario","arguments":{"name":"rtp_loss","service_id":"zdf-hd"}}`),
	})

	if response.Error != nil {
		t.Fatalf("satip_set_scenario error: %+v", response.Error)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method: got %q", gotMethod)
	}
	if !strings.Contains(gotBody.String(), `"name":"rtp_loss"`) || !strings.Contains(gotBody.String(), `"service_id":"zdf-hd"`) {
		t.Fatalf("body: %s", gotBody.String())
	}
}

func mustRaw(raw string) json.RawMessage {
	return json.RawMessage(raw)
}

func mustDecodeResult(t *testing.T, response Response, target any) {
	t.Helper()
	body, err := json.Marshal(response.Result)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatal(err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
