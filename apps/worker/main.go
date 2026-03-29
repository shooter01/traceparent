package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

type response struct {
	Service     string `json:"service"`
	Status      string `json:"status"`
	DelayMs     int    `json:"delay_ms"`
	Traceparent string `json:"traceparent,omitempty"`
	RequestID   string `json:"x_request_id,omitempty"`
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		delayMs, _ := strconv.Atoi(r.URL.Query().Get("delay_ms"))
		fail := r.URL.Query().Get("fail") == "1"

		if delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}

		resp := response{
			Service:     "worker",
			Status:      "ok",
			DelayMs:     delayMs,
			Traceparent: r.Header.Get("traceparent"),
			RequestID:   r.Header.Get("x-request-id"),
		}

		w.Header().Set("Content-Type", "application/json")

		if fail {
			resp.Status = "failed"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(resp)
	})

	log.Println("worker listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
