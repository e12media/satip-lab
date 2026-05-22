package simctl

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunStatusPrintsSimulatorStatus(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tuners":[{"id":1,"state":"idle"}],"sessions":[],"events":[]}`))
	}))
	defer api.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--http-url", api.URL, "status"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"tuners"`) {
		t.Fatalf("stdout: %s", stdout.String())
	}
}

func TestRunScenarioPostsNameAndTarget(t *testing.T) {
	var method, body string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		body = buf.String()
		if r.URL.Path != "/api/scenario" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"rtp_loss","service_id":"zdf-hd"}`))
	}))
	defer api.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--http-url", api.URL, "scenario", "rtp_loss", "--service", "zdf-hd"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if method != http.MethodPost {
		t.Fatalf("method: got %q", method)
	}
	for _, want := range []string{`"name":"rtp_loss"`, `"service_id":"zdf-hd"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
	if !strings.Contains(stdout.String(), `"rtp_loss"`) {
		t.Fatalf("stdout: %s", stdout.String())
	}
}

func TestRunScenarioHelpPrintsUsageWithoutCallingAPI(t *testing.T) {
	called := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer api.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--http-url", api.URL, "scenario", "--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if called {
		t.Fatal("scenario --help should not call the API")
	}
	if !strings.Contains(stdout.String(), "Usage: satip-labctl [--http-url URL] scenario") {
		t.Fatalf("stdout: %s", stdout.String())
	}
}

func TestRunWaitPollsUntilReady(t *testing.T) {
	attempts := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/context" {
			t.Fatalf("path: got %q", r.URL.Path)
		}
		attempts++
		if attempts == 1 {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"version":"1.0"}`))
	}))
	defer api.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--http-url", api.URL, "wait", "--timeout", "1s", "--interval", "1ms"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code: got %d stderr=%s", code, stderr.String())
	}
	if attempts != 2 {
		t.Fatalf("attempts: got %d", attempts)
	}
	if !strings.Contains(stdout.String(), "satip-lab ready") {
		t.Fatalf("stdout: %s", stdout.String())
	}
}
