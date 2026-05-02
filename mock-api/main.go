package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Receipt struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	IssuedAt    time.Time `json:"issued_at"`
	Status      string    `json:"status"`
}

var receipts = []Receipt{
	{ID: "rcpt-001", ClientID: "client-123", Amount: 1500.00, Currency: "BRL", Description: "Tax receipt Q1 2026", IssuedAt: time.Now().AddDate(0, -3, 0), Status: "issued"},
	{ID: "rcpt-002", ClientID: "client-456", Amount: 3200.50, Currency: "BRL", Description: "Tax receipt Q4 2025", IssuedAt: time.Now().AddDate(0, -6, 0), Status: "issued"},
	{ID: "rcpt-003", ClientID: "client-789", Amount: 800.75, Currency: "BRL", Description: "Tax receipt Q3 2025", IssuedAt: time.Now().AddDate(0, -9, 0), Status: "issued"},
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleReceipts(w http.ResponseWriter, r *http.Request) {
	// Route: /receipts or /receipts/{id}
	path := strings.TrimPrefix(r.URL.Path, "/receipts")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		writeJSON(w, http.StatusOK, receipts)
		return
	}

	for _, rcpt := range receipts {
		if rcpt.ID == path {
			writeJSON(w, http.StatusOK, rcpt)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "receipt not found", "id": path})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mock-api"})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("invalid PORT: %s", port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/receipts", handleReceipts)
	mux.HandleFunc("/receipts/", handleReceipts)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("mock-api listening on %s", addr)
	if err := http.ListenAndServe(addr, loggingMiddleware(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
