package http

import (
	"encoding/json"
	"net/http"

	"github.com/tomek7667/links/internal/domain"
)

func (s *Server) AddIndexRoute() {
	s.r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		links := s.dber.GetLinks()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := indexTmpl.Execute(w, links); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	s.r.Post("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var link domain.Link
		if err := json.NewDecoder(r.Body).Decode(&link); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.SaveLink(link)
		w.WriteHeader(http.StatusCreated)
	})

	s.r.Delete("/api/links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Url string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.dber.DeleteLink(req.Url)
		w.WriteHeader(http.StatusOK)
	})

	s.r.Get("/api/resources", func(w http.ResponseWriter, r *http.Request) {
		if s.resources == nil {
			http.Error(w, "resources not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		withHistory := r.URL.Query().Get("history") == "1"
		if err := json.NewEncoder(w).Encode(s.resources.Snapshot(withHistory)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
