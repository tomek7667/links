package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tomek7667/links/internal/domain"
)

const (
	maxBodyBytes   = 1 << 20  // 1 MiB
	maxImportBytes = 10 << 20 // 10 MiB
)

func (s *Server) addRoutes() {
	s.r.Get("/", s.handleIndex)
	s.r.Get("/assets/*", assetsHandler())

	s.r.Route("/api", func(r chi.Router) {
		r.Get("/links", s.handleListLinks)
		r.Post("/links", s.handleCreateLink)
		r.Delete("/links", s.handleDeleteLinkByURL) // legacy: {"url": "..."}
		r.Put("/links/order", s.handleReorderLinks)
		r.Post("/links/replace-host", s.handleReplaceHost)
		r.Get("/links/status", s.handleLinkStatus)
		r.Put("/links/{id}", s.handleUpdateLink)
		r.Delete("/links/{id}", s.handleDeleteLink)

		r.Post("/groups/rename", s.handleRenameGroup)

		r.Get("/settings", s.handleGetSettings)
		r.Put("/settings", s.handleSaveSettings)

		r.Get("/export", s.handleExport)
		r.Post("/import", s.handleImport)

		r.Get("/resources", s.handleResources)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrInvalid):
		code = http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func readJSON(w http.ResponseWriter, r *http.Request, limit int64, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("%w: malformed JSON body: %v", domain.ErrInvalid, err)
	}
	return nil
}

// requestHostname is what {host} resolves to: the hostname the browser used.
func requestHostname(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.Trim(host, "[]")
}

// ---------- page ----------

type initialState struct {
	Links    []domain.Link   `json:"links"`
	Settings domain.Settings `json:"settings"`
	HostIP   string          `json:"hostIp"`
	Hostname string          `json:"hostname"`
}

type indexData struct {
	Title   string
	Version string
	State   initialState
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	settings := s.dber.Settings()
	title := settings.Title
	if title == "" {
		title = "Links"
	}
	data := indexData{
		Title:   title,
		Version: assetVersion,
		State: initialState{
			Links:    s.dber.Links(),
			Settings: settings,
			HostIP:   s.effectiveHostIP(),
			Hostname: s.resources.Hostname(),
		},
	}

	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// ---------- links ----------

func (s *Server) handleListLinks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.dber.Links())
}

func (s *Server) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	var link domain.Link
	if err := readJSON(w, r, maxBodyBytes, &link); err != nil {
		writeError(w, err)
		return
	}
	at := -1
	if v := r.URL.Query().Get("at"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			at = n
		}
	}
	created, err := s.dber.CreateLink(link, at)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateLink(w http.ResponseWriter, r *http.Request) {
	var link domain.Link
	if err := readJSON(w, r, maxBodyBytes, &link); err != nil {
		writeError(w, err)
		return
	}
	updated, err := s.dber.UpdateLink(chi.URLParam(r, "id"), link)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteLink(w http.ResponseWriter, r *http.Request) {
	if err := s.dber.DeleteLink(chi.URLParam(r, "id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteLinkByURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Url string `json:"url"`
	}
	if err := readJSON(w, r, maxBodyBytes, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.dber.DeleteLinkByURL(req.Url); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReorderLinks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(w, r, maxBodyBytes, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := s.dber.ReorderLinks(req.IDs); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.dber.Links())
}

func (s *Server) handleReplaceHost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := readJSON(w, r, maxBodyBytes, &req); err != nil {
		writeError(w, err)
		return
	}
	n, err := s.dber.ReplaceHost(req.From, req.To)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": n, "links": s.dber.Links()})
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := readJSON(w, r, maxBodyBytes, &req); err != nil {
		writeError(w, err)
		return
	}
	n, err := s.dber.RenameGroup(req.From, req.To)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": n, "links": s.dber.Links()})
}

func (s *Server) handleLinkStatus(w http.ResponseWriter, r *http.Request) {
	ip := s.effectiveHostIP()
	host := requestHostname(r)
	if host == "" {
		host = ip
	}
	writeJSON(w, http.StatusOK, s.checker.CheckAll(r.Context(), s.dber.Links(), ip, host))
}

// ---------- settings ----------

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.dber.Settings())
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var settings domain.Settings
	if err := readJSON(w, r, maxBodyBytes, &settings); err != nil {
		writeError(w, err)
		return
	}
	saved, err := s.dber.SaveSettings(settings)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

// ---------- backup ----------

type backup struct {
	Version    int             `json:"version"`
	ExportedAt string          `json:"exportedAt"`
	Links      []domain.Link   `json:"links"`
	Settings   domain.Settings `json:"settings"`
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	name := "links-backup-" + time.Now().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	writeJSON(w, http.StatusOK, backup{
		Version:    1,
		ExportedAt: time.Now().Format(time.RFC3339),
		Links:      s.dber.Links(),
		Settings:   s.dber.Settings(),
	})
}

// handleImport accepts an export file ({"links": [...]}), a bare array of
// links, or a links.db.json from any version. ?replace=1 drops existing links.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxImportBytes))
	if err != nil {
		writeError(w, fmt.Errorf("%w: %v", domain.ErrInvalid, err))
		return
	}
	body = bytes.TrimSpace(body)

	var links []domain.Link
	if bytes.HasPrefix(body, []byte("[")) {
		err = json.Unmarshal(body, &links)
	} else {
		var b backup
		err = json.Unmarshal(body, &b)
		links = b.Links
	}
	if err != nil {
		writeError(w, fmt.Errorf("%w: not a valid backup file: %v", domain.ErrInvalid, err))
		return
	}

	replace := r.URL.Query().Get("replace") == "1"
	added, skipped, err := s.dber.ImportLinks(links, replace)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "skipped": skipped, "links": s.dber.Links()})
}

// ---------- resources ----------

func (s *Server) handleResources(w http.ResponseWriter, r *http.Request) {
	if s.resources == nil {
		http.Error(w, "resources not available", http.StatusServiceUnavailable)
		return
	}
	snap := s.resources.Snapshot(r.URL.Query().Get("history") == "1")
	if o := s.dber.Settings().HostIP; o != "" {
		snap.HostIP = o
		snap.HostIPOverride = true
	}
	writeJSON(w, http.StatusOK, snap)
}
