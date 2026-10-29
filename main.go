
package main

import (
    "encoding/json"
	"fmt"
    "log"
    "net/http"
    "sync"
)

type Item struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var (
    items = map[int]Item{}
    mu    sync.Mutex
)

func main() {
    http.HandleFunc("/items", itemsHandler)

    log.Println("REST API running on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func itemsHandler(w http.ResponseWriter, r *http.Request) {
    mu.Lock()
    defer mu.Unlock()

    w.Header().Set("Content-Type", "application/json")

    switch r.Method {
    case http.MethodGet:
        result := make([]Item, 0, len(items))
        for _, item := range items {
            result = append(result, item)
        }
        json.NewEncoder(w).Encode(result)

    case http.MethodPost:
        var item Item
        if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        if item.ID <= 0 || item.Name == "" {
            http.Error(w, "Invalid item", http.StatusBadRequest)
            return
        }
        if _, exists := items[item.ID]; exists {
            http.Error(w, "Item already exists", http.StatusConflict)
            return
        }
        items[item.ID] = item
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(item)

    case http.MethodPut:
        var item Item
        if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        if item.ID <= 0 || item.Name == "" {
            http.Error(w, "Invalid item", http.StatusBadRequest)
            return
        }
        if _, exists := items[item.ID]; !exists {
            http.Error(w, "Item not found", http.StatusNotFound)
            return
        }
        items[item.ID] = item
        json.NewEncoder(w).Encode(item)

    case http.MethodDelete:
        id := r.URL.Query().Get("id")
        if id == "" {
            http.Error(w, "Missing id", http.StatusBadRequest)
            return
        }
        var itemID int
        if _, err := fmt.Sscanf(id, "%d", &itemID); err != nil {
            http.Error(w, "Invalid id", http.StatusBadRequest)
            return
        }
        if _, exists := items[itemID]; !exists {
            http.Error(w, "Item not found", http.StatusNotFound)
            return
        }
        delete(items, itemID)
        w.WriteHeader(http.StatusNoContent)

    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}
