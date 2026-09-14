package main

import (
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

	for _, path := range []string{"/", "/latency", "/traffic"} {
		t.Run(path, func(t *testing.T) {
			response, err := http.Get(server.URL + path)
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
		})
	}

	for _, path := range []string{"/app.js", "/style.css"} {
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
