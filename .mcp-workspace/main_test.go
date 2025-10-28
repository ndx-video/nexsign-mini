package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingHandler(t *testing.T) {
	mux := newMux()
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	res := w.Result()
	body, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if string(body) != "pong" {
		t.Fatalf("expected body 'pong', got %s", string(body))
	}
}

func TestRPCInitialize(t *testing.T) {
	mux := newMux()
	payload := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]interface{}{}}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/rpc", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	res := w.Result()
	body, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	var r map[string]interface{}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("invalid json response: %v", err)
	}
	if r["result"] == nil {
		t.Fatalf("expected result field in response")
	}
}
