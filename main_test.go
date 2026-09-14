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
		{path: "/", present: "latency-chart-svg"},
		{path: "/latency", present: "latency-chart-svg", absent: "traffic-chart-svg"},
		{path: "/traffic", present: "traffic-chart-svg", absent: "latency-chart-svg"},
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
		"/latency/app.js",
		"/latency/view.js",
		"/traffic/app.js",
		"/traffic/view.js",
		"/shared/base.css",
		"/shared/chart.js",
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
