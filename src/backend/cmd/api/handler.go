package main

import (
	"encoding/json"
	"net/http"
)

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
