package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otel-gitverse-demo/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type hookRequest struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Event string `json:"event"`
}

type hookServer struct {
	tracer trace.Tracer
}

func main() {
	ctx := context.Background()
	shutdownTelemetry, err := telemetry.Init(ctx, "hook-receiver")
	if err != nil {
		log.Fatalf("init telemetry: %v", err)
	}
	defer func() {
		_ = shutdownTelemetry(context.Background())
	}()

	srv := &hookServer{tracer: telemetry.Tracer("hook-receiver")}

	mux := http.NewServeMux()
	mux.HandleFunc("/hook", srv.handleHook)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"service": "hook-receiver"})
	})

	handler := telemetry.Middleware("hook-receiver", srv.tracer)(mux)
	httpServer := &http.Server{
		Addr:              ":8081",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("hook-receiver listening on :8081")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("hook server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown hook server: %v", err)
	}
}

func (s *hookServer) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	var payload hookRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	ctx = s.runStep(ctx, "hook.validate", 30*time.Millisecond,
		attribute.String("repo.owner", payload.Owner),
		attribute.String("repo.name", payload.Name),
		attribute.String("event.name", payload.Event),
	)
	ctx = s.runStep(ctx, "hook.store_event", 60*time.Millisecond,
		attribute.String("event.name", payload.Event),
	)

	currentSpan := trace.SpanFromContext(ctx)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      "accepted",
		"trace_id":    currentSpan.SpanContext().TraceID().String(),
		"traceparent": r.Header.Get("traceparent"),
	})
}

func (s *hookServer) runStep(ctx context.Context, name string, d time.Duration, attrs ...attribute.KeyValue) context.Context {
	ctx, span := s.tracer.Start(ctx, name)
	defer span.End()
	span.SetAttributes(attrs...)
	time.Sleep(d)
	return ctx
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
