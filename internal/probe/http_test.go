package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPProbeAcceptsSuccessfulResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	rtt, err := newHTTPProbeWithClient(server.Client()).Probe(context.Background(), server.URL, time.Second)
	if err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
	if rtt < 0 {
		t.Fatalf("RTT = %v", rtt)
	}
}

func TestHTTPProbeRejectsErrorStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := newHTTPProbeWithClient(server.Client()).Probe(context.Background(), server.URL, time.Second)
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("Probe() error = %v, want status 503", err)
	}
}

func TestHTTPProbeHonorsTimeout(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	_, err := newHTTPProbeWithClient(server.Client()).Probe(context.Background(), server.URL, 10*time.Millisecond)
	if err == nil {
		t.Fatal("Probe() error = nil, want timeout")
	}
}

func TestHTTPProbeHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newHTTPProbe().Probe(ctx, "https://example.com", time.Second)
	if err == nil {
		t.Fatal("Probe() error = nil, want cancellation")
	}
}
