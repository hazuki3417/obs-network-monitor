package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"obs-network-monitor/internal/adapter"
	"obs-network-monitor/internal/config"
	"obs-network-monitor/internal/monitor"
	"obs-network-monitor/internal/probe"
)

//go:embed web/*
var webFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func websocketHandler(source *monitor.Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade: %v", err)
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

func main() {
	appConfig, err := config.LoadFromExecutable(context.Background())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	networkMonitor := monitor.New(
		adapter.NewInspector(),
		probe.NewEngine(appConfig.ICMPTarget, appConfig.HTTPTarget),
		appConfig.ICMPTarget,
		log.Default(),
	)
	go networkMonitor.Run(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", websocketHandler(networkMonitor))
	mux.Handle("/", http.FileServer(http.FS(static)))

	addr := "127.0.0.1:8080"
	log.Printf("OBS Network Monitor: http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
