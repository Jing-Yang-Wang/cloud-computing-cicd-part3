
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func resetItems() {
	mu.Lock()
	defer mu.Unlock()
	items = map[int]Item{}
}

func sendRequest(method, body string) *httptest.ResponseRecorder {
	var req *http.Request

	if body == "" {
		req = httptest.NewRequest(method, "/items", nil)
	} else {
		req = httptest.NewRequest(method, "/items",
			bytes.NewBufferString(body))
	}

	rec := httptest.NewRecorder()
	itemsHandler(rec, req)
	return rec
}

func TestGET(t *testing.T) {
	resetItems()

	rec := sendRequest(http.MethodGet, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d", rec.Code)
	}

	var result []Item
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
}

func TestPOST(t *testing.T) {
	resetItems()

	rec := sendRequest(http.MethodPost,
		`{"id":1,"name":"Apple"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: expected 201, got %d", rec.Code)
	}
}

func TestPUT(t *testing.T) {
	resetItems()

	sendRequest(http.MethodPost,
		`{"id":1,"name":"Apple"}`)

	rec := sendRequest(http.MethodPut,
		`{"id":1,"name":"Banana"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: expected 200, got %d", rec.Code)
	}

	mu.Lock()
	updated := items[1]
	mu.Unlock()

	if updated.Name != "Banana" {
		t.Fatalf("PUT: expected Banana, got %s", updated.Name)
	}
}

func TestDELETE(t *testing.T) {
	resetItems()

	sendRequest(http.MethodPost,
		`{"id":1,"name":"Apple"}`)

	req := httptest.NewRequest(http.MethodDelete, "/items?id=1", nil)
	rec := httptest.NewRecorder()
	itemsHandler(rec, req)	

	if rec.Code != http.StatusOK &&
		rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: expected 200 or 204, got %d", rec.Code)
	}

	mu.Lock()
	_, exists := items[1]
	mu.Unlock()

	if exists {
		t.Fatal("DELETE: item still exists")
	}
}
