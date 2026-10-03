package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestWithRouteSetsMatchedPattern(t *testing.T) {
	tests := []struct {
		method, path, wantRoute string
	}{
		{http.MethodGet, "/health", "/health"},
		{http.MethodPost, "/items/42", "/items/{id}"},
		{http.MethodGet, "/mcp", "/mcp"},
		{http.MethodGet, "/nope", ""},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

			mux := http.NewServeMux()
			mux.HandleFunc("GET /health", handleHealth)
			mux.HandleFunc("POST /items/{id}", func(http.ResponseWriter, *http.Request) {})
			mux.HandleFunc("/mcp", func(http.ResponseWriter, *http.Request) {})

			h := otelhttp.NewHandler(withRoute(mux), "http.server", otelhttp.WithTracerProvider(tp))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tt.method, tt.path, nil))

			spans := rec.Ended()
			if len(spans) != 1 {
				t.Fatalf("got %d spans, want 1", len(spans))
			}
			var got string
			for _, kv := range spans[0].Attributes() {
				if kv.Key == "http.route" {
					got = kv.Value.AsString()
				}
			}
			if got != tt.wantRoute {
				t.Errorf("http.route = %q, want %q", got, tt.wantRoute)
			}
		})
	}
}
