package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otel-gitverse-demo/internal/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type createRepoRequest struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type hookRequest struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Event string `json:"event"`
}

type server struct {
	tracer  trace.Tracer
	client  *http.Client
	hookURL string
}

func main() {
	ctx := context.Background()
	shutdownTelemetry, err := telemetry.Init(ctx, "gitverse-api")
	if err != nil {
		log.Fatalf("init telemetry: %v", err)
	}
	defer func() {
		_ = shutdownTelemetry(context.Background())
	}()

	srv := &server{
		tracer:  telemetry.Tracer("gitverse-api"),
		client:  &http.Client{Timeout: 5 * time.Second},
		hookURL: envOrDefault("HOOK_URL", "http://hook:8081/hook"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/repos", srv.handleCreateRepo)

	handler := telemetry.Middleware("gitverse-api", srv.tracer)(mux)
	httpServer := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("gitverse-api listening on :8080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("api server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown api server: %v", err)
	}
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service": "gitverse-api",
		"hint":    "POST /repos with JSON {\"owner\":\"alice\",\"name\":\"demo\"}",
	})
}

func (s *server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	currentSpan := trace.SpanFromContext(ctx)

	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		currentSpan.RecordError(err)
		currentSpan.SetStatus(codes.Error, "invalid json")
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.Owner == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "owner and name are required"})
		return
	}

	repoID := fmt.Sprintf("%s-%s-%d", req.Owner, req.Name, time.Now().UnixMilli())

	s.runStep(ctx, "auth.check", 40*time.Millisecond,
		attribute.String("repo.owner", req.Owner),
		attribute.String("repo.name", req.Name),
	)
	ctx = s.runStep(ctx, "db.insert_repo", 70*time.Millisecond,
		attribute.String("repo.id", repoID),
		attribute.String("repo.owner", req.Owner),
		attribute.String("repo.name", req.Name),
	)
	ctx = s.runStep(ctx, "git.init_repo", 120*time.Millisecond,
		attribute.String("repo.id", repoID),
	)

	if err := s.sendHook(ctx, hookRequest{Owner: req.Owner, Name: req.Name, Event: "repo.created"}); err != nil {
		currentSpan.RecordError(err)
		currentSpan.SetStatus(codes.Error, "hook delivery failed")
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    "hook delivery failed",
			"trace_id": currentSpan.SpanContext().TraceID().String(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":   "created",
		"repo_id":  repoID,
		"owner":    req.Owner,
		"name":     req.Name,
		"trace_id": currentSpan.SpanContext().TraceID().String(),
	})
}

func (s *server) runStep(ctx context.Context, name string, d time.Duration, attrs ...attribute.KeyValue) context.Context {
	ctx, span := s.tracer.Start(ctx, name)
	defer span.End()
	span.SetAttributes(attrs...)
	time.Sleep(d)
	return ctx
}

func (s *server) sendHook(ctx context.Context, payload hookRequest) error {
	ctx, span := s.tracer.Start(ctx, "hook.deliver", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	body, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hookURL, bytes.NewReader(body))
	if err != nil {
		span.RecordError(err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := s.client.Do(req)
	if err != nil {
		span.RecordError(err)
		return err
	}
	defer resp.Body.Close()

	span.SetAttributes(
		attribute.String("http.url", s.hookURL),
		attribute.Int("http.response.status_code", resp.StatusCode),
		attribute.String("traceparent", req.Header.Get("traceparent")),
	)

	if resp.StatusCode >= http.StatusBadRequest {
		err := fmt.Errorf("hook responded with %s", resp.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
