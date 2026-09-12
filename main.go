package main

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"obs-network-monitor/internal/config"
)

//go:embed web/*
var webFS embed.FS

type Status struct {
	Online    bool   `json:"online"`
	LatencyMS int64  `json:"latencyMs"`
	CheckedAt string `json:"checkedAt"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func checkNetwork(target string) Status {
	start := time.Now()
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(target)
	latency := time.Since(start).Milliseconds()

	online := err == nil
	if resp != nil {
		resp.Body.Close()
		online = online && resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	return Status{
		Online:    online,
		LatencyMS: latency,
		CheckedAt: time.Now().Format(time.RFC3339),
	}
}

func websocketHandler(httpTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade: %v", err)
			return
		}
		defer conn.Close()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			status := checkNetwork(httpTarget)
			payload, _ := json.Marshal(status)
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
			<-ticker.C
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

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", websocketHandler(appConfig.HTTPTarget))
	mux.Handle("/", http.FileServer(http.FS(static)))

	addr := "127.0.0.1:8080"
	log.Printf("OBS Network Monitor: http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
