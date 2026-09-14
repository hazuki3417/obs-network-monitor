package main

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisplayHandler(t *testing.T) {
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		t.Fatal(err)
	}
	displays, err := displayHandler(static)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(displays)
	defer server.Close()

	pages := []struct {
		path    string
		present string
		absent  string
	}{
		{path: "/", present: "service-status", absent: "chart-canvas"},
		{path: "/latency", present: "latency-chart-canvas", absent: "traffic-chart-canvas"},
		{path: "/traffic", present: "traffic-chart-canvas", absent: "latency-chart-canvas"},
		{path: "/route", present: "route-nodes", absent: "chart-canvas"},
	}
	for _, page := range pages {
		t.Run(page.path, func(t *testing.T) {
			response, err := http.Get(server.URL + page.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
			if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
				t.Fatalf("Content-Type = %q, want text/html", contentType)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), page.present) {
				t.Fatalf("body does not contain %q", page.present)
			}
			if page.absent != "" && strings.Contains(string(body), page.absent) {
				t.Fatalf("body unexpectedly contains %q", page.absent)
			}
		})
	}

	for _, path := range []string{
		"/app.js",
		"/home.css",
		"/latency/app.js",
		"/latency/view.js",
		"/traffic/app.js",
		"/traffic/view.js",
		"/route/app.js",
		"/route/view.js",
		"/route/route.css",
		"/shared/animation.js",
		"/shared/base.css",
		"/shared/chart.js",
		"/shared/dom.js",
		"/shared/options.js",
		"/shared/websocket.js",
	} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
			}
		})
	}

	response, err := http.Get(server.URL + "/unknown")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestShutdownHandlerRejectsUnsafeRequests(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		host         string
		origin       string
		header       string
		wantStatus   int
		wantShutdown bool
	}{
		{
			name:         "same origin POST",
			method:       http.MethodPost,
			host:         listenAddress,
			origin:       "http://" + listenAddress,
			header:       "1",
			wantStatus:   http.StatusAccepted,
			wantShutdown: true,
		},
		{name: "GET", method: http.MethodGet, host: listenAddress, wantStatus: http.StatusMethodNotAllowed},
		{name: "different host", method: http.MethodPost, host: "localhost:8080", origin: "http://localhost:8080", header: "1", wantStatus: http.StatusForbidden},
		{name: "different origin", method: http.MethodPost, host: listenAddress, origin: "https://example.com", header: "1", wantStatus: http.StatusForbidden},
		{name: "missing header", method: http.MethodPost, host: listenAddress, origin: "http://" + listenAddress, wantStatus: http.StatusForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shutdown := false
			handler := shutdownHandler(listenAddress, func() { shutdown = true })
			request := httptest.NewRequest(test.method, "http://"+listenAddress+"/api/shutdown", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.header != "" {
				request.Header.Set(shutdownRequestHeader, test.header)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if shutdown != test.wantShutdown {
				t.Fatalf("shutdown = %v, want %v", shutdown, test.wantShutdown)
			}
		})
	}
}
