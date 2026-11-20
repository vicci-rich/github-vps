package proof

import (
	"errors"
	"net/http"
	"strings"

	"route/internal/xmlio"
)

// Handler serves GET /proof/{sha} as an XML <proof> document.
func Handler(s *Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodOptions {
			w.Header().Set("Allow", "GET, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		sha := strings.TrimPrefix(r.URL.Path, "/proof/")
		if sha == "" || strings.Contains(sha, "/") {
			http.Error(w, "expected /proof/{sha}", http.StatusBadRequest)
			return
		}
		doc, err := s.Build(sha)
		if err != nil {
			switch {
			case errors.Is(err, ErrNotFound):
				http.NotFound(w, r)
			case strings.Contains(err.Error(), "not a commit sha"):
				http.Error(w, err.Error(), http.StatusBadRequest)
			default:
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			}
			return
		}
		body, err := xmlio.Marshal(doc)
		if err != nil {
			http.Error(w, "encode", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}
