// Package server serves the page and the JSON API. ⚠ Both render the same
// diag.Result and compute nothing of their own (docs/adr/0006).
package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hidetzu/connect-doctor/internal/diag"
	"github.com/hidetzu/connect-doctor/internal/limits"
)

//go:embed templates/page.html
var templates embed.FS

// Server is the HTTP front of a diag.Checker.
type Server struct {
	checker *diag.Checker
	log     *log.Logger
	page    *template.Template
	slots   chan struct{}
	busy    atomic.Int64 // requests answered "busy"; counted, never shown to users
}

// New returns a Server. Logs go to logger.
func New(c *diag.Checker, logger *log.Logger) *Server {
	return newWithSlots(c, logger, limits.ConcurrentChecks)
}

func newWithSlots(c *diag.Checker, logger *log.Logger, slots int) *Server {
	page := template.Must(template.New("page.html").Funcs(template.FuncMap{
		"mark":    mark,
		"label":   diag.StatusLabel,
		"upper":   strings.ToUpper,
		"inc":     func(i int) int { return i + 1 },
		"message": diag.Message,
	}).ParseFS(templates, "templates/page.html"))
	return &Server{checker: c, log: logger, page: page, slots: make(chan struct{}, slots)}
}

// Busy returns how many requests were answered "busy" so far.
func (s *Server) Busy() int64 { return s.busy.Load() }

// Handler returns the routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /api/check", s.handleAPI)
	return s.logRequests(mux)
}

// acquire takes a check slot without waiting. ⚠ Reaching the bound is
// answered as busy, never as a failure of the target
// (.claude/rules/security.md § 4).
func (s *Server) acquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		s.busy.Add(1)
		return false
	}
}

func (s *Server) release() { <-s.slots }

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if !s.acquire() {
		var e apiError
		e.Error.Code, e.Error.Message = "server.busy", diag.Message("server.busy")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(e)
		return
	}
	res := s.checker.Check(r.Context(), r.URL.Query().Get("url"))
	s.release()

	status := http.StatusOK
	if strings.HasPrefix(res.Conclusion.Code, "input.") {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

type pageData struct {
	Note   string
	Input  string
	Result *diag.Result
	Busy   string
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// The page runs no script and loads nothing from elsewhere.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")

	d := pageData{Note: diag.ObservedFromNote}
	if _, asked := r.URL.Query()["url"]; asked {
		d.Input = r.URL.Query().Get("url")
		if s.acquire() {
			res := s.checker.Check(r.Context(), d.Input)
			s.release()
			d.Result = &res
		} else {
			d.Busy = diag.Message("server.busy")
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
	if err := s.page.Execute(w, d); err != nil {
		s.log.Printf("page: %v", err)
	}
}

func mark(st diag.Status) string {
	switch st {
	case diag.StatusOK:
		return "✅"
	case diag.StatusFailed:
		return "❌"
	case diag.StatusRefused:
		return "⛔"
	case diag.StatusNotImplemented:
		return "🚧"
	}
	return "—"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// logRequests writes one line per request.
// ⚠ The path only: a query string carries the URL being checked, and URLs
// carry tokens (.claude/rules/security.md § 5). This is the one place a log
// line about a request is built.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Printf("%s %s %d %dms", r.Method, r.URL.Path, sw.status, time.Since(start).Milliseconds())
	})
}
