// Package ticker serves the two market-data endpoints the client reads:
// /markets.json and /assets.json. It is the single market-data survivor of the
// duplication audit; the fixture file mirrors what bds-xlm's ticker returns.
package ticker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Data is the fixture shape: two arrays served verbatim.
type Data struct {
	Markets []json.RawMessage `json:"markets"`
	Assets  []json.RawMessage `json:"assets"`
}

// Load reads and validates a fixture file.
func Load(path string) (*Data, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ticker: %w", err)
	}
	var d Data
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("ticker: %s: %w", path, err)
	}
	if d.Markets == nil {
		d.Markets = []json.RawMessage{}
	}
	if d.Assets == nil {
		d.Assets = []json.RawMessage{}
	}
	return &d, nil
}

// Handler returns the mux for the two endpoints. CORS is permissive because
// the client is served from a different origin during development.
func Handler(d *Data) http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, payload []json.RawMessage) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		body, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "encode", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(append(body, '\n'))
	}
	mux.HandleFunc("/markets.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		write(w, d.Markets)
	})
	mux.HandleFunc("/assets.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		write(w, d.Assets)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return mux
}
