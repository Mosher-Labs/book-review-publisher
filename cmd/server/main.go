package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mosher-labs/book-review-publisher/internal/auth"
	"github.com/mosher-labs/book-review-publisher/internal/publisher"
	"github.com/mosher-labs/book-review-publisher/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.Setup(ctx, "book-review-publisher")
	if err != nil {
		slog.Error("telemetry setup failed", "error", err)
		os.Exit(1)
	}

	pat := os.Getenv("GITHUB_PAT")
	if pat == "" {
		slog.Error("GITHUB_PAT environment variable is required")
		os.Exit(1)
	}

	authToken := os.Getenv("AUTH_TOKEN")
	if authToken == "" {
		slog.Error("AUTH_TOKEN environment variable is required")
		os.Exit(1)
	}

	oauthClientID := os.Getenv("OAUTH_CLIENT_ID")
	if oauthClientID == "" {
		slog.Error("OAUTH_CLIENT_ID environment variable is required")
		os.Exit(1)
	}

	pub := publisher.New(pat)
	oauth := auth.New(oauthClientID, "", authToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /publish", requireBearer(authToken, pub.HandlePublish))
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", oauth.HandleMeta)
	mux.HandleFunc("GET /authorize", oauth.HandleAuthorize)
	mux.HandleFunc("POST /token", oauth.HandleToken)
	mux.Handle("/mcp", requireBearer(authToken, buildMCPHandler(pub).ServeHTTP))

	// otelhttp records a server span per request; withRoute adds the
	// matched route so request metrics can be grouped by endpoint.
	srv := &http.Server{ //nolint:gosec
		Addr:    ":8080",
		Handler: otelhttp.NewHandler(withRoute(mux), "http.server"),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown failed", "error", err)
		}
	}()

	slog.Info("starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	// Flush buffered spans before exiting.
	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdownTracing(flushCtx); err != nil {
		slog.Error("telemetry shutdown failed", "error", err)
	}
}

func buildMCPHandler(pub *publisher.Publisher) http.Handler {
	s := server.NewMCPServer(
		"book-review-publisher",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	publishTool := mcp.NewTool("publish_book_review",
		mcp.WithDescription("Publishes a book review to benniemosher.com by committing the markdown post and cover image, then opening a pull request."),
		mcp.WithString("markdown",
			mcp.Required(),
			mcp.Description(`The full Jekyll post markdown. Use exactly this front matter format:

---
layout: post
title: "Book Title Here"
date: YYYY-MM-DD
categories:
  - book reviews
description: One-sentence description for the listing page excerpt.
---

Then write the review body. Rules:
- Do NOT repeat the title as a heading — the layout renders it.
- Do NOT include a markdown image for the cover — the publisher injects it.
- START the body with a real opening paragraph (1-3 sentences). This becomes
  the excerpt shown on the home page listing. Put ratings/metadata after it.
- Use a "---" horizontal rule to separate the metadata block from the review.
- Format book metadata as a markdown table:

| | |
|---|---|
| **Author** | Name |
| **Format** | Kindle / Paperback / etc. |
| **Read** | dates |
| **Era** | timeline info |
| **Series** | series name |

- Write the rest of the review as normal paragraphs after the metadata.`),
		),
		mcp.WithString("image_url",
			mcp.Required(),
			mcp.Description("Public URL of the book cover image to commit to assets/img/book-covers/"),
		),
	)

	s.AddTool(publishTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		markdown, err := req.RequireString("markdown")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		imageURL, err := req.RequireString("image_url")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		prURL, err := pub.Publish(ctx, markdown, imageURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("publish failed: %s", err.Error())), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Published! PR: %s", prURL)), nil
	})

	return server.NewStreamableHTTPServer(s)
}

// withRoute sets http.route on the request's span to the ServeMux pattern
// that matched ("GET /publish" -> "/publish"), never the raw path, so IDs in
// a URL can't create new metric series. Unmatched requests get no route.
func withRoute(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if r.Pattern == "" {
			return
		}
		route := r.Pattern
		if _, path, ok := strings.Cut(route, " "); ok {
			route = path
		}
		trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
	})
}

func requireBearer(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || bearer != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("encode health response failed", "error", err)
	}
}
