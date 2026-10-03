// Package httpapi exposes the shortener over HTTP with JSON bodies.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/D0CCi/go-shortener-service/internal/shortener"
	"github.com/D0CCi/go-shortener-service/internal/storage"
)

// maxBodyBytes fits a 4096-character url (up to 2 bytes per Cyrillic letter) plus JSON.
const maxBodyBytes = 8 << 10

// Shortener is what the handlers need from the service.
type Shortener interface {
	Shorten(ctx context.Context, rawURL string) (string, string, bool, error)
	Resolve(ctx context.Context, code string) (string, error)
}

type handler struct {
	svc     Shortener
	baseURL string // prefix for short links, e.g. "http://localhost:8080"
	log     *slog.Logger
}

type shortenRequest struct {
	URL string `json:"url"`
}

// linkResponse is returned by both POST and GET, so a link always looks the same.
type linkResponse struct {
	URL      string `json:"url"`
	ShortURL string `json:"short_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// New returns an http.Handler with the API routes.
func New(svc Shortener, baseURL string, log *slog.Logger) http.Handler {
	h := &handler{
		svc:     svc,
		baseURL: baseURL,
		log:     log,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /{$}", h.shorten) // {$} matches only "/", not "/anything"
	mux.HandleFunc("GET /{code}", h.resolve)
	return jsonFallback{mux: mux}
}

// jsonFallback makes the mux's own 404 and 405 replies JSON, like every other error.
type jsonFallback struct {
	mux *http.ServeMux
}

func (f jsonFallback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, pattern := f.mux.Handler(r)
	if pattern != "" {
		f.mux.ServeHTTP(w, r) // a route matched; the mux also fills r.PathValue
		return
	}
	// No route: the mux decides between 404 and 405 and sets the Allow header,
	// we keep its status and replace the plain-text body.
	rec := &statusRecorder{header: w.Header()}
	h.ServeHTTP(rec, r)
	writeError(w, rec.status, strings.ToLower(http.StatusText(rec.status)))
}

// statusRecorder remembers the status and drops the body.
type statusRecorder struct {
	header http.Header
	status int
}

func (s *statusRecorder) Header() http.Header         { return s.header }
func (s *statusRecorder) WriteHeader(status int)      { s.status = status }
func (s *statusRecorder) Write(b []byte) (int, error) { return len(b), nil }

func (h *handler) shorten(w http.ResponseWriter, r *http.Request) {
	req := shortenRequest{}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes) // a huge body fails Decode instead of filling memory
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	code, link, created, err := h.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	status := http.StatusOK // the url was already shortened before
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, linkResponse{URL: link, ShortURL: h.shortURL(code)})
}

func (h *handler) resolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	link, err := h.svc.Resolve(r.Context(), code)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, linkResponse{URL: link, ShortURL: h.shortURL(code)})
}

func (h *handler) shortURL(code string) string {
	return h.baseURL + "/" + code
}

// writeJSON sends v as JSON. Headers and status must go before the body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // status is already sent, nothing to do on error
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeServiceError maps service errors to HTTP statuses.
// Unknown errors become 500 without details, so internals do not leak to clients.
func (h *handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, shortener.ErrInvalidURL), errors.Is(err, shortener.ErrInvalidCode):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		h.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
