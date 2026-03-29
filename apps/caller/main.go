package main

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/url"
)

func ensureRequestID(r *http.Request) string {
	if rid := r.Header.Get("x-request-id"); rid != "" {
		return rid
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/call", func(w http.ResponseWriter, r *http.Request) {
		workerURL := "http://worker.demo.svc.cluster.local/work"

		q := url.Values{}
		q.Set("delay_ms", r.URL.Query().Get("delay_ms"))
		q.Set("fail", r.URL.Query().Get("fail"))

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, workerURL+"?"+q.Encode(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Pass tracing and correlation headers through to the downstream service.
		for _, h := range []string{"traceparent", "tracestate", "baggage", "x-request-id"} {
			if v := r.Header.Get(h); v != "" {
				req.Header.Set(h, v)
			}
		}
		if req.Header.Get("x-request-id") == "" {
			req.Header.Set("x-request-id", ensureRequestID(r))
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	log.Println("caller listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
