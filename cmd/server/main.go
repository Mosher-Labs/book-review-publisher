package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/mosher-labs/book-review-publisher/internal/publisher"
)

func main() {
	pat := os.Getenv("GITHUB_PAT")
	if pat == "" {
		slog.Error("GITHUB_PAT environment variable is required")
		os.Exit(1)
	}

	pub := publisher.New(pat)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /publish", pub.HandlePublish)

	addr := ":8080"
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("encode health response failed", "error", err)
	}
}
