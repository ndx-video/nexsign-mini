package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// newMux returns the HTTP handler mux so tests can reuse it.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", rpcHandler)
	mux.HandleFunc("/ping", pingHandler)
	mux.HandleFunc("/ready", readyHandler)
	return mux
}

func rpcHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Error: "invalid JSON"})
		return
	}

	switch req.Method {
	case "initialize":
		// Minimal initialize reply - MCP/initialize style
		res := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"capabilities": []string{"mcp.server"},
			},
		}
		json.NewEncoder(w).Encode(res)
	case "ping":
		res := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: "pong"}
		json.NewEncoder(w).Encode(res)
	default:
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: "method not supported"})
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("pong"))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	// Simple readiness endpoint. Always ready in this scaffold.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ready"))
}

func main() {
	// Read port from env (default 4000)
	port := 4000
	if p := os.Getenv("MCP_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	addr := ":" + strconv.Itoa(port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      newMux(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("MCP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down MCP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}
