package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"obs-network-monitor/internal/adapter"
	"obs-network-monitor/internal/config"
	"obs-network-monitor/internal/errorlog"
	"obs-network-monitor/internal/monitor"
	"obs-network-monitor/internal/probe"
	"obs-network-monitor/internal/traceroute"
)

//go:embed web
var webFS embed.FS

const (
	listenAddress         = "127.0.0.1:8080"
	shutdownRequestHeader = "X-OBS-Network-Monitor-Shutdown"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type logger interface {
	Printf(format string, values ...any)
}

func websocketHandler(source *monitor.Monitor, errors logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errors.Printf("websocket upgrade: %v", err)
			return
		}
		defer conn.Close()

		updates, unsubscribe := source.Subscribe()
		defer unsubscribe()
		for snapshot := range updates {
			if err := conn.WriteJSON(snapshot); err != nil {
				return
			}
		}
	}
}

func shutdownHandler(expectedHost string, requestShutdown func()) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		expectedOrigin := "http://" + expectedHost
		if r.Host != expectedHost ||
			r.Header.Get("Origin") != expectedOrigin ||
			r.Header.Get(shutdownRequestHeader) != "1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"shutting-down"}`))
		requestShutdown()
	}
}

func displayHandler(static fs.FS) (http.Handler, error) {
	pageFiles := map[string]string{
		"/":         "index.html",
		"/latency":  "latency/index.html",
		"/latency/": "latency/index.html",
		"/traffic":  "traffic/index.html",
		"/traffic/": "traffic/index.html",
		"/route":    "route/index.html",
		"/route/":   "route/index.html",
	}
	pages := make(map[string][]byte, len(pageFiles))
	for path, name := range pageFiles {
		content, err := fs.ReadFile(static, name)
		if err != nil {
			return nil, err
		}
		pages[path] = content
	}
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if page, ok := pages[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
			return
		}
		files.ServeHTTP(w, r)
	}), nil
}

func run(errorLogger logger) error {
	ctx, cancelMonitor := context.WithCancel(context.Background())
	defer cancelMonitor()

	appConfig, err := config.LoadFromExecutable(ctx)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("load embedded web UI: %w", err)
	}
	networkMonitor := monitor.New(
		adapter.NewInspector(),
		probe.NewEngine(appConfig.ICMPTarget, appConfig.HTTPTarget),
		appConfig.ICMPTarget,
		errorLogger,
	)
	networkMonitor.ConfigureTraceroute(
		traceroute.New(appConfig.TracerouteTarget()),
		time.Duration(appConfig.Traceroute.IntervalSeconds)*time.Second,
		appConfig.Traceroute.MaxNodes,
	)
	go networkMonitor.Run(ctx)

	shutdownRequests := make(chan struct{})
	var shutdownOnce sync.Once
	requestShutdown := func() {
		shutdownOnce.Do(func() { close(shutdownRequests) })
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", websocketHandler(networkMonitor, errorLogger))
	mux.HandleFunc("/api/shutdown", shutdownHandler(listenAddress, requestShutdown))
	displays, err := displayHandler(static)
	if err != nil {
		return fmt.Errorf("load display pages: %w", err)
	}
	mux.Handle("/", displays)

	server := &http.Server{Addr: listenAddress, Handler: mux}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	case <-shutdownRequests:
		cancelMonitor()
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("stop http server: %w", err)
		}
		return nil
	}
}

func executableLogDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return "logs"
	}
	return filepath.Join(filepath.Dir(executable), "logs")
}

func main() {
	errorLogger := errorlog.New(executableLogDirectory())
	if err := run(errorLogger); err != nil {
		errorLogger.Printf("%v", err)
		os.Exit(1)
	}
}
